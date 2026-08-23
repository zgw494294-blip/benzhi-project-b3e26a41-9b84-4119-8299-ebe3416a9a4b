package workflow

import (
	"context"
	"errors"
	"testing"
	"time"

	"benzhi-project-b3e26a41-9b84-4119-8299-ebe3416a9a4b/internal/domain"
	"benzhi-project-b3e26a41-9b84-4119-8299-ebe3416a9a4b/internal/storage"
)

func TestServiceRoleAndProjection(t *testing.T) {
	store, err := storage.Open(context.Background(), storage.Options{Path: ":memory:"})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	now := time.Now().UTC()
	service := New(store, Options{Clock: func() time.Time { return now }, IDGenerator: func(prefix string) string { return prefix + "-fixed" }})
	_, _, err = service.CreateBatch(context.Background(), CreateBatchCommand{RequestID: "role-1", Role: RoleReviewer, Input: domain.BatchInput{SourceLab: "实验室", OwnerName: "安全员", PlannedHandoverAt: now.Add(time.Hour)}})
	var rule *domain.RuleError
	if !errors.As(err, &rule) || rule.Code != "forbidden" {
		t.Fatalf("expected forbidden create, got %v", err)
	}
	batch, _, err := service.CreateBatch(context.Background(), CreateBatchCommand{RequestID: "role-2", Role: RoleSafetyOfficer, Input: domain.BatchInput{SourceLab: "实验室", OwnerName: "安全员", PlannedHandoverAt: now.Add(time.Hour)}})
	if err != nil {
		t.Fatal(err)
	}
	projection, err := service.Projection(context.Background(), batch.ID, RoleSafetyOfficer)
	if err != nil || projection.Batch.ID != batch.ID || len(projection.AllowedActions) != 3 {
		t.Fatalf("unexpected projection: %#v err=%v", projection, err)
	}
}
