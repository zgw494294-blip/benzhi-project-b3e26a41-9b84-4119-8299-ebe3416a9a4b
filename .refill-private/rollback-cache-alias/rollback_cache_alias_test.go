package rollback_cache_alias_test

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"benzhi-project-b3e26a41-9b84-4119-8299-ebe3416a9a4b/internal/domain"
	"benzhi-project-b3e26a41-9b84-4119-8299-ebe3416a9a4b/internal/storage"
	"benzhi-project-b3e26a41-9b84-4119-8299-ebe3416a9a4b/internal/workflow"
)

func TestFailedCorrectionDoesNotPolluteCachedBatch(t *testing.T) {
	ctx := context.Background()
	store, err := storage.Open(ctx, storage.Options{Path: filepath.Join(t.TempDir(), "handover.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	now := time.Date(2026, time.August, 24, 9, 0, 0, 0, time.UTC)
	sequence := 0
	service := workflow.New(store, workflow.Options{
		Clock: func() time.Time { return now },
		IDGenerator: func(prefix string) string {
			sequence++
			return fmt.Sprintf("%s-%d", prefix, sequence)
		},
	})

	batch, _, err := service.CreateBatch(ctx, workflow.CreateBatchCommand{
		RequestID: "create", Role: workflow.RoleSafetyOfficer,
		Input: domain.BatchInput{SourceLab: "缓存边界实验室", OwnerName: "王安全", PlannedHandoverAt: now.Add(48 * time.Hour)},
	})
	if err != nil {
		t.Fatal(err)
	}
	safetyMeta := func(requestID string, version int64) workflow.CommandMeta {
		return workflow.CommandMeta{RequestID: requestID, ExpectedVersion: version, Role: workflow.RoleSafetyOfficer, Actor: "王安全"}
	}
	reviewerMeta := func(requestID string, version int64) workflow.CommandMeta {
		return workflow.CommandMeta{RequestID: requestID, ExpectedVersion: version, Role: workflow.RoleReviewer, Actor: "李复核"}
	}

	originalName := "待整改废液"
	batch, _, err = service.AddItem(ctx, workflow.AddItemCommand{
		BatchID: batch.ID, Meta: safetyMeta("add", batch.Version),
		Input: domain.ItemInput{MaterialName: originalName, HazardClasses: []string{"toxic"}, Quantity: 1, Unit: "L", DisposalCategory: "liquid"},
	})
	if err != nil {
		t.Fatal(err)
	}
	itemID := batch.Items[0].ID
	batch, _, err = service.SetPackage(ctx, workflow.PackageCommand{
		BatchID: batch.ID, ItemID: itemID, Meta: safetyMeta("package", batch.Version),
		Input: domain.PackageInput{ContainerType: "HDPE 桶", SealChecked: true, LabelChecked: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	batch, _, err = service.SubmitReview(ctx, workflow.SubmitCommand{BatchID: batch.ID, Meta: safetyMeta("submit", batch.Version)})
	if err != nil {
		t.Fatal(err)
	}
	batch, _, err = service.DecideReview(ctx, workflow.ReviewCommand{
		BatchID: batch.ID, ItemID: itemID, Meta: reviewerMeta("reject", batch.Version),
		Input: domain.ReviewInput{Decision: domain.DecisionReject, ReasonCode: domain.ReasonPackage, Comment: "需要更换容器", ReviewerName: "李复核"},
	})
	if err != nil {
		t.Fatal(err)
	}
	batch, _, err = service.CompleteReview(ctx, workflow.SubmitCommand{BatchID: batch.ID, Meta: reviewerMeta("complete", batch.Version)})
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = service.CorrectItem(ctx, workflow.CorrectionCommand{
		BatchID: batch.ID, ItemID: itemID, Meta: safetyMeta("failed-correction", batch.Version),
		Item:    domain.ItemInput{MaterialName: "未提交的新名称", HazardClasses: []string{"toxic"}, Quantity: 1, Unit: "L", DisposalCategory: "liquid"},
		Package: domain.PackageInput{}, Note: "尝试整改但封装参数缺失",
	})
	if err == nil {
		t.Fatal("expected correction validation error")
	}

	listed, err := service.ListBatches(ctx, 10, 0)
	if err != nil || len(listed) != 1 || listed[0].Items[0].MaterialName != originalName {
		t.Fatalf("database state changed after rollback: batches=%#v err=%v", listed, err)
	}
	fetched, err := service.GetBatch(ctx, batch.ID)
	if err != nil {
		t.Fatal(err)
	}
	if fetched.Items[0].MaterialName != originalName {
		t.Fatalf("failed correction polluted cached batch: got %q want %q", fetched.Items[0].MaterialName, originalName)
	}
}
