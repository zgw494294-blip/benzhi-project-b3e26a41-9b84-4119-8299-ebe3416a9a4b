package canceled_update_commits_test

import (
	"context"
	"testing"
	"time"

	"benzhi-project-b3e26a41-9b84-4119-8299-ebe3416a9a4b/internal/domain"
	"benzhi-project-b3e26a41-9b84-4119-8299-ebe3416a9a4b/internal/storage"
)

func TestCanceledUpdateDoesNotCommit(t *testing.T) {
	store, err := storage.Open(context.Background(), storage.Options{Path: ":memory:"})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	now := time.Date(2026, time.August, 24, 10, 0, 0, 0, time.UTC)
	batch, err := domain.NewBatch("cancel-batch", domain.BatchInput{
		SourceLab: "取消测试实验室", OwnerName: "安全员", PlannedHandoverAt: now.Add(24 * time.Hour),
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.CreateBatch(context.Background(), "create-cancel-batch", batch); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	mutationStarted := make(chan struct{})
	continueMutation := make(chan struct{})
	result := make(chan error, 1)
	go func() {
		_, _, updateErr := store.UpdateBatch(ctx, batch.ID, batch.Version, "canceled-add-item", "add_item", func(current *domain.HandoverBatch) error {
			close(mutationStarted)
			<-continueMutation
			return current.AddItem("canceled-item", domain.ItemInput{
				MaterialName: "取消后不应保存的废液", HazardClasses: []string{"toxic"},
				Quantity: 1, Unit: "L", DisposalCategory: "liquid",
			}, "安全员", now.Add(time.Minute))
		})
		result <- updateErr
	}()

	<-mutationStarted
	cancel()
	close(continueMutation)
	updateErr := <-result
	fetched, err := store.GetBatch(context.Background(), batch.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updateErr == nil || len(fetched.Items) != 0 || fetched.Version != batch.Version {
		t.Fatalf("canceled update committed batch mutation: updateErr=%v items=%d version=%d", updateErr, len(fetched.Items), fetched.Version)
	}
}
