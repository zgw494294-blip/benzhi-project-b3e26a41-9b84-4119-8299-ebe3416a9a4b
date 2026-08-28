package corruptbatch_test

import (
	"context"
	"database/sql"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"benzhi-project-b3e26a41-9b84-4119-8299-ebe3416a9a4b/internal/domain"
	"benzhi-project-b3e26a41-9b84-4119-8299-ebe3416a9a4b/internal/httpapi"
	"benzhi-project-b3e26a41-9b84-4119-8299-ebe3416a9a4b/internal/storage"
	"benzhi-project-b3e26a41-9b84-4119-8299-ebe3416a9a4b/internal/workflow"
	_ "modernc.org/sqlite"
)

func TestCorruptBatchReadReturnsInternalError(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "handover.db")
	store, err := storage.Open(ctx, storage.Options{Path: dbPath})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 24, 9, 0, 0, 0, time.UTC)
	batch, err := domain.NewBatch("batch-corrupt", domain.BatchInput{
		SourceLab: "化学实验室", OwnerName: "安全员", PlannedHandoverAt: now.Add(24 * time.Hour),
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.CreateBatch(ctx, "create-corrupt", batch); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, "UPDATE batches SET data_json = ? WHERE id = ?", []byte(`{"id":`), batch.ID); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	store, err = storage.Open(ctx, storage.Options{Path: dbPath})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	service := workflow.New(store, workflow.Options{Clock: func() time.Time { return now }})
	server := httptest.NewServer(httpapi.New(service, nil).Handler())
	defer server.Close()

	req, err := http.NewRequest(http.MethodGet, server.URL+"/api/batches/"+batch.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Role", "safety_officer")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusInternalServerError || !strings.Contains(string(body), `"code":"internal_error"`) {
		t.Fatalf("损坏批次应返回 500 internal_error，得到 status=%d body=%s", resp.StatusCode, body)
	}
}
