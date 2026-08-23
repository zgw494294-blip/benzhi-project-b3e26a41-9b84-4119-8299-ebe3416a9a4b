package storage

import (
	"context"
	"errors"
	"testing"
	"time"

	"benzhi-project-b3e26a41-9b84-4119-8299-ebe3416a9a4b/internal/domain"
)

func newStoredBatch(t *testing.T, store *Store) *domain.HandoverBatch {
	t.Helper()
	now := time.Now().UTC().Add(24 * time.Hour)
	batch, err := domain.NewBatch("stored-batch", domain.BatchInput{SourceLab: "存储实验室", OwnerName: "存储安全员", PlannedHandoverAt: now}, now)
	if err != nil {
		t.Fatal(err)
	}
	created, replayed, err := store.CreateBatch(context.Background(), "create-1", batch)
	if err != nil || replayed || created.ID != batch.ID {
		t.Fatalf("create failed: batch=%#v replayed=%v err=%v", created, replayed, err)
	}
	return created
}

func TestStoreVersionAndIdempotency(t *testing.T) {
	store, err := Open(context.Background(), Options{Path: ":memory:"})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	batch := newStoredBatch(t, store)
	itemInput := domain.ItemInput{MaterialName: "固体废物", HazardClasses: []string{"toxic"}, Quantity: 2, Unit: "kg", DisposalCategory: "solid"}
	updated, replayed, err := store.UpdateBatch(context.Background(), batch.ID, batch.Version, "item-1", "add_item", func(current *domain.HandoverBatch) error {
		return current.AddItem("item-1", itemInput, "安全员", time.Now().UTC())
	})
	if err != nil || replayed || len(updated.Items) != 1 {
		t.Fatalf("update failed: %#v replayed=%v err=%v", updated, replayed, err)
	}
	replay, replayed, err := store.UpdateBatch(context.Background(), batch.ID, batch.Version, "item-1", "add_item", func(current *domain.HandoverBatch) error {
		return errors.New("mutation must not execute during replay")
	})
	if err != nil || !replayed || replay.Version != updated.Version {
		t.Fatalf("replay failed: %#v replayed=%v err=%v", replay, replayed, err)
	}
	_, _, err = store.UpdateBatch(context.Background(), batch.ID, batch.Version, "stale-1", "stale", func(current *domain.HandoverBatch) error { return nil })
	var conflict *domain.ConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("expected version conflict, got %v", err)
	}
	fetched, err := store.GetBatch(context.Background(), batch.ID)
	if err != nil || fetched.Version != updated.Version || len(fetched.Items) != 1 {
		t.Fatalf("fetched batch mismatch: %#v err=%v", fetched, err)
	}
}

func TestStoreRejectsClosedAndListsBatches(t *testing.T) {
	store, err := Open(context.Background(), Options{Path: ":memory:"})
	if err != nil {
		t.Fatal(err)
	}
	newStoredBatch(t, store)
	list, err := store.ListBatches(context.Background(), 10, 0)
	if err != nil || len(list) != 1 {
		t.Fatalf("list failed: %#v err=%v", list, err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetBatch(context.Background(), "stored-batch"); !errors.Is(err, ErrClosed) {
		t.Fatalf("expected ErrClosed, got %v", err)
	}
}
