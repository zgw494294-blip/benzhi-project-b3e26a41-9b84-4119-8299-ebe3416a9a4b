package dateoffsetduestatus

import (
	"context"
	"testing"
	"time"

	"benzhi-project-b3e26a41-9b84-4119-8299-ebe3416a9a4b/internal/domain"
	"benzhi-project-b3e26a41-9b84-4119-8299-ebe3416a9a4b/internal/storage"
	"benzhi-project-b3e26a41-9b84-4119-8299-ebe3416a9a4b/internal/workflow"
)

func TestPositiveOffsetDateRemainsDueSoon(t *testing.T) {
	ctx := context.Background()
	store, err := storage.Open(ctx, storage.Options{Path: ":memory:"})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	now := time.Date(2026, time.August, 24, 0, 0, 0, 0, time.UTC)
	zone := time.FixedZone("UTC+14", 14*60*60)
	planned := time.Date(2026, time.August, 25, 12, 0, 0, 0, zone)
	service := workflow.New(store, workflow.Options{
		Clock:       func() time.Time { return now },
		IDGenerator: func(prefix string) string { return prefix + "-date-offset" },
	})
	batch, _, err := service.CreateBatch(ctx, workflow.CreateBatchCommand{
		RequestID: "create-date-offset", Role: workflow.RoleSafetyOfficer,
		Input: domain.BatchInput{SourceLab: "日期实验室", OwnerName: "安全员", PlannedHandoverAt: planned},
	})
	if err != nil {
		t.Fatal(err)
	}
	projection, err := service.Projection(ctx, batch.ID, workflow.RoleSafetyOfficer)
	if err != nil {
		t.Fatal(err)
	}
	soon, err := service.ListBatches(ctx, 20, 0, domain.DueSoon)
	if err != nil {
		t.Fatal(err)
	}
	if projection.DueStatus != domain.DueSoon || len(soon) != 1 {
		t.Fatalf("positive offset date lost: projection=%s dueSoonCount=%d stored=%s", projection.DueStatus, len(soon), batch.PlannedHandoverAt.Format(time.RFC3339))
	}
}
