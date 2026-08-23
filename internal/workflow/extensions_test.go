package workflow

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"benzhi-project-b3e26a41-9b84-4119-8299-ebe3416a9a4b/internal/domain"
	"benzhi-project-b3e26a41-9b84-4119-8299-ebe3416a9a4b/internal/storage"
)

func TestBulkCommandsRescheduleIdempotencyAndConflict(t *testing.T) {
	ctx := context.Background()
	store, err := storage.Open(ctx, storage.Options{Path: ":memory:"})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	now := time.Date(2026, time.August, 23, 8, 0, 0, 0, time.UTC)
	sequence := 0
	service := New(store, Options{Clock: func() time.Time { return now }, IDGenerator: func(prefix string) string { sequence++; return fmt.Sprintf("%s-%d", prefix, sequence) }})
	batch, _, err := service.CreateBatch(ctx, CreateBatchCommand{RequestID: "create", Role: RoleSafetyOfficer, Input: domain.BatchInput{SourceLab: "批量实验室", OwnerName: "王安全", PlannedHandoverAt: now.Add(24 * time.Hour)}})
	if err != nil {
		t.Fatal(err)
	}
	meta := func(id string, version int64) CommandMeta {
		return CommandMeta{RequestID: id, ExpectedVersion: version, Role: RoleSafetyOfficer, Actor: "王安全"}
	}
	for index := 0; index < 2; index++ {
		batch, _, err = service.AddItem(ctx, AddItemCommand{BatchID: batch.ID, Meta: meta(fmt.Sprintf("add-%d", index), batch.Version), Input: domain.ItemInput{MaterialName: fmt.Sprintf("废液%d", index), HazardClasses: []string{"toxic"}, Quantity: 1, Unit: "L", DisposalCategory: "liquid"}})
		if err != nil {
			t.Fatal(err)
		}
	}
	changes := []domain.PackageChange{}
	for _, item := range batch.Items {
		changes = append(changes, domain.PackageChange{ItemID: item.ID, Input: domain.PackageInput{ContainerType: "HDPE 桶", SealChecked: true, LabelChecked: true}})
	}
	version := batch.Version
	bulkMeta := meta("packages", version)
	batch, replayed, err := service.SetPackagesBulk(ctx, PackageBulkCommand{BatchID: batch.ID, Meta: bulkMeta, Changes: changes})
	if err != nil || replayed || batch.Version != version+1 {
		t.Fatalf("bulk result version=%d replayed=%v err=%v", batch.Version, replayed, err)
	}
	replayedBatch, replayed, err := service.SetPackagesBulk(ctx, PackageBulkCommand{BatchID: batch.ID, Meta: bulkMeta, Changes: changes})
	if err != nil || !replayed || replayedBatch.Version != batch.Version {
		t.Fatalf("replay failed: %#v %v", replayedBatch, err)
	}

	reschedule := RescheduleCommand{BatchID: batch.ID, Meta: meta("reschedule", batch.Version), PlannedHandoverAt: now.Add(72 * time.Hour), Reason: "接收单位调整排期"}
	batch, _, err = service.Reschedule(ctx, reschedule)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = service.Reschedule(ctx, RescheduleCommand{BatchID: batch.ID, Meta: meta("stale-reschedule", batch.Version-1), PlannedHandoverAt: now.Add(96 * time.Hour), Reason: "再次调整排期"})
	var conflict *domain.ConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("expected version conflict, got %v", err)
	}
	stored, err := service.GetBatch(ctx, batch.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !stored.PlannedHandoverAt.Equal(now.Add(72 * time.Hour)) {
		t.Fatal("stale request overwrote schedule")
	}
	batch = stored
	batch, _, err = service.SubmitReview(ctx, SubmitCommand{BatchID: batch.ID, Meta: meta("submit", batch.Version)})
	if err != nil {
		t.Fatal(err)
	}
	reviewerMeta := func(id string, version int64) CommandMeta {
		return CommandMeta{RequestID: id, ExpectedVersion: version, Role: RoleReviewer, Actor: "李复核"}
	}
	duplicate := []domain.ReviewChange{
		{ItemID: batch.Items[0].ID, Input: domain.ReviewInput{Decision: domain.DecisionApprove, ReviewerName: "李复核"}},
		{ItemID: batch.Items[0].ID, Input: domain.ReviewInput{Decision: domain.DecisionApprove, ReviewerName: "李复核"}},
	}
	_, _, err = service.DecideReviewsBulk(ctx, ReviewBulkCommand{BatchID: batch.ID, Meta: reviewerMeta("review-duplicate", batch.Version), Changes: duplicate})
	var rule *domain.RuleError
	if !errors.As(err, &rule) || rule.Code != "duplicate_item" {
		t.Fatalf("expected duplicate error, got %v", err)
	}
	stored, err = service.GetBatch(ctx, batch.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Version != batch.Version || len(stored.Reviews) != 0 {
		t.Fatal("invalid bulk review mutated batch")
	}
	reviews := []domain.ReviewChange{
		{ItemID: batch.Items[0].ID, Input: domain.ReviewInput{Decision: domain.DecisionApprove, ReviewerName: "李复核"}},
		{ItemID: batch.Items[1].ID, Input: domain.ReviewInput{Decision: domain.DecisionReject, ReasonCode: domain.ReasonQuantity, Comment: "数量记录需复核", ReviewerName: "李复核"}},
	}
	batch, _, err = service.DecideReviewsBulk(ctx, ReviewBulkCommand{BatchID: batch.ID, Meta: reviewerMeta("review-valid", batch.Version), Changes: reviews})
	if err != nil || len(batch.Reviews) != 2 || batch.Version != stored.Version+1 {
		t.Fatalf("bulk review failed: %#v %v", batch, err)
	}

	overdue, err := service.ListBatches(ctx, 20, 0, domain.DueOverdue)
	if err != nil || len(overdue) != 0 {
		t.Fatalf("unexpected overdue results: %d %v", len(overdue), err)
	}
	soon, err := service.ListBatches(ctx, 20, 0, domain.DueSoon)
	if err != nil || len(soon) != 1 || soon[0].DueStatus != domain.DueSoon {
		t.Fatalf("unexpected due-soon results: %#v %v", soon, err)
	}
}
