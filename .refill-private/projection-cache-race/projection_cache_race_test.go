package projection_cache_race_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"benzhi-project-b3e26a41-9b84-4119-8299-ebe3416a9a4b/internal/domain"
	"benzhi-project-b3e26a41-9b84-4119-8299-ebe3416a9a4b/internal/storage"
	"benzhi-project-b3e26a41-9b84-4119-8299-ebe3416a9a4b/internal/workflow"
)

func TestConcurrentProjectionCacheAccess(t *testing.T) {
	ctx := context.Background()
	store, err := storage.Open(ctx, storage.Options{Path: ":memory:"})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	service := workflow.New(store, workflow.Options{Clock: func() time.Time {
		return time.Date(2026, time.August, 24, 9, 0, 0, 0, time.UTC)
	}})
	batch, _, err := service.CreateBatch(ctx, workflow.CreateBatchCommand{
		RequestID: "projection-race-create",
		Role:      workflow.RoleSafetyOfficer,
		Input: domain.BatchInput{
			SourceLab:         "并发投影实验室",
			OwnerName:         "安全员",
			PlannedHandoverAt: time.Date(2026, time.August, 25, 9, 0, 0, 0, time.UTC),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	batch, _, err = store.UpdateBatch(ctx, batch.ID, batch.Version, "projection-race-items", "seed_projection_items", func(current *domain.HandoverBatch) error {
		for index := 0; index < 200; index++ {
			if err := current.AddItem(
				"projection-item-"+time.Date(2026, time.August, 24, 0, 0, index, 0, time.UTC).Format("150405.000000000"),
				domain.ItemInput{MaterialName: "并发测试废物", HazardClasses: []string{"toxic"}, Quantity: 1, Unit: "kg", DisposalCategory: "solid"},
				"安全员", time.Date(2026, time.August, 24, 9, 0, 0, 0, time.UTC)); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	const workers = 64
	start := make(chan struct{})
	var ready sync.WaitGroup
	var done sync.WaitGroup
	ready.Add(workers)
	done.Add(workers)
	for index := 0; index < workers; index++ {
		go func() {
			defer done.Done()
			ready.Done()
			<-start
			if _, err := service.Projection(ctx, batch.ID, workflow.RoleSafetyOfficer); err != nil {
				t.Errorf("投影读取失败: %v", err)
			}
		}()
	}
	ready.Wait()
	close(start)
	done.Wait()
}
