package association_sync_after_commit_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"benzhi-project-b3e26a41-9b84-4119-8299-ebe3416a9a4b/internal/domain"
	"benzhi-project-b3e26a41-9b84-4119-8299-ebe3416a9a4b/internal/storage"
	"benzhi-project-b3e26a41-9b84-4119-8299-ebe3416a9a4b/internal/workflow"
)

func TestFailedAssociationSyncDoesNotCommitBatch(t *testing.T) {
	ctx := context.Background()
	store, err := storage.Open(ctx, storage.Options{Path: ":memory:"})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	var idMu sync.Mutex
	sequence := 0
	ids := func(prefix string) string {
		idMu.Lock()
		defer idMu.Unlock()
		if prefix == "item" {
			return "item-collision"
		}
		sequence++
		return fmt.Sprintf("%s-%d", prefix, sequence)
	}
	now := time.Date(2026, time.August, 24, 9, 0, 0, 0, time.UTC)
	service := workflow.New(store, workflow.Options{Clock: func() time.Time { return now }, IDGenerator: ids})

	create := func(requestID, lab string) *domain.HandoverBatch {
		t.Helper()
		batch, replayed, createErr := service.CreateBatch(ctx, workflow.CreateBatchCommand{
			RequestID: requestID,
			Role:      workflow.RoleSafetyOfficer,
			Input: domain.BatchInput{
				SourceLab: lab, OwnerName: "安全员", PlannedHandoverAt: now.Add(24 * time.Hour),
			},
		})
		if createErr != nil || replayed {
			t.Fatalf("create batch: replayed=%v err=%v", replayed, createErr)
		}
		return batch
	}
	first := create("create-first", "实验室 A")
	second := create("create-second", "实验室 B")
	item := domain.ItemInput{
		MaterialName: "含丙酮废液", HazardClasses: []string{"flammable"},
		Quantity: 1, Unit: "L", DisposalCategory: "liquid",
	}
	meta := func(requestID string, version int64) workflow.CommandMeta {
		return workflow.CommandMeta{
			RequestID: requestID, ExpectedVersion: version,
			Role: workflow.RoleSafetyOfficer, Actor: "安全员",
		}
	}
	if _, _, err := service.AddItem(ctx, workflow.AddItemCommand{
		BatchID: first.ID, Meta: meta("add-first", first.Version), Input: item,
	}); err != nil {
		t.Fatal(err)
	}

	_, replayed, err := service.AddItem(ctx, workflow.AddItemCommand{
		BatchID: second.ID, Meta: meta("add-second", second.Version), Input: item,
	})
	if err == nil || replayed || !strings.Contains(err.Error(), "保存条目投影") {
		t.Fatalf("collision did not reach association sync: replayed=%v err=%v", replayed, err)
	}

	stored, err := service.GetBatch(ctx, second.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Version != second.Version || len(stored.Items) != 0 {
		t.Fatalf("failed association sync committed batch mutation: version=%d items=%d", stored.Version, len(stored.Items))
	}

	_, replayed, err = service.AddItem(ctx, workflow.AddItemCommand{
		BatchID: second.ID, Meta: meta("add-second", second.Version), Input: item,
	})
	if err == nil || replayed {
		t.Fatalf("failed association sync committed batch mutation: retry replayed=%v err=%v", replayed, err)
	}
}
