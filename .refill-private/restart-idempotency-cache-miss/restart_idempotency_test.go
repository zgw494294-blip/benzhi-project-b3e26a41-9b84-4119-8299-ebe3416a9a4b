package restartcache_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"benzhi-project-b3e26a41-9b84-4119-8299-ebe3416a9a4b/internal/httpapi"
	"benzhi-project-b3e26a41-9b84-4119-8299-ebe3416a9a4b/internal/storage"
	"benzhi-project-b3e26a41-9b84-4119-8299-ebe3416a9a4b/internal/workflow"
)

type createResponse struct {
	Data struct {
		ID string `json:"id"`
	} `json:"data"`
	Meta struct {
		Replayed bool `json:"replayed"`
	} `json:"meta"`
}

func fixedIDs(instance string) workflow.IDGenerator {
	sequence := 0
	return func(prefix string) string {
		sequence++
		return fmt.Sprintf("%s-%s-%d", prefix, instance, sequence)
	}
}

func createBatch(handler http.Handler) *httptest.ResponseRecorder {
	body := `{"requestId":"restart-create","sourceLab":"重启实验室","ownerName":"王安全","plannedHandoverAt":"2026-08-25T08:00:00Z"}`
	request := httptest.NewRequest(http.MethodPost, "/api/batches", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Role", "safety_officer")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

func TestRestartedStoreReplaysPersistedRequest(t *testing.T) {
	databasePath := t.TempDir() + "/handover.db"
	now := time.Date(2026, time.August, 24, 8, 0, 0, 0, time.UTC)

	firstStore, err := storage.Open(context.Background(), storage.Options{Path: databasePath})
	if err != nil {
		t.Fatal(err)
	}
	firstService := workflow.New(firstStore, workflow.Options{Clock: func() time.Time { return now }, IDGenerator: fixedIDs("first")})
	first := createBatch(httpapi.New(firstService, nil).Handler())
	if first.Code != http.StatusCreated {
		t.Fatalf("initial create status=%d body=%s", first.Code, first.Body.String())
	}
	var original createResponse
	if err := json.Unmarshal(first.Body.Bytes(), &original); err != nil {
		t.Fatal(err)
	}
	if err := firstStore.Close(); err != nil {
		t.Fatal(err)
	}

	restartedStore, err := storage.Open(context.Background(), storage.Options{Path: databasePath})
	if err != nil {
		t.Fatal(err)
	}
	defer restartedStore.Close()
	restartedService := workflow.New(restartedStore, workflow.Options{Clock: func() time.Time { return now }, IDGenerator: fixedIDs("restarted")})
	replayed := createBatch(httpapi.New(restartedService, nil).Handler())
	if replayed.Code != http.StatusOK {
		t.Fatalf("restart replay returned status=%d body=%s", replayed.Code, replayed.Body.String())
	}
	var result createResponse
	if err := json.Unmarshal(replayed.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if !result.Meta.Replayed || result.Data.ID != original.Data.ID {
		t.Fatalf("restart replay changed result: original=%s replayed=%s meta=%v", original.Data.ID, result.Data.ID, result.Meta.Replayed)
	}
}
