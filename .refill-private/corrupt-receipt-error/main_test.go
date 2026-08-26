package corruptreceipterror

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"benzhi-project-b3e26a41-9b84-4119-8299-ebe3416a9a4b/internal/domain"
	"benzhi-project-b3e26a41-9b84-4119-8299-ebe3416a9a4b/internal/httpapi"
	"benzhi-project-b3e26a41-9b84-4119-8299-ebe3416a9a4b/internal/storage"
	"benzhi-project-b3e26a41-9b84-4119-8299-ebe3416a9a4b/internal/workflow"
)

func TestCorruptReceiptErrorClassification(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "handover.db")
	store, err := storage.Open(context.Background(), storage.Options{Path: databasePath})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	now := time.Now().UTC()
	batch, err := domain.NewBatch("corrupt-receipt-batch", domain.BatchInput{
		SourceLab: "损坏凭据实验室", OwnerName: "安全员", PlannedHandoverAt: now.Add(24 * time.Hour),
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.CreateBatch(context.Background(), "create-corrupt-receipt", batch); err != nil {
		t.Fatal(err)
	}

	corruptor, err := sql.Open("sqlite", databasePath)
	if err != nil {
		t.Fatal(err)
	}
	_, err = corruptor.Exec(`INSERT INTO archive_receipts(id,batch_id,manifest_digest,timeline_digest,receipt_json,issued_at) VALUES(?,?,?,?,?,?)`,
		"receipt-corrupt", batch.ID, "manifest", "timeline", []byte("{"), now.Format(time.RFC3339Nano))
	if err != nil {
		corruptor.Close()
		t.Fatal(err)
	}
	if err := corruptor.Close(); err != nil {
		t.Fatal(err)
	}

	api := httpapi.New(workflow.New(store, workflow.Options{}), nil)
	request := httptest.NewRequest(http.MethodGet, "/api/batches/"+batch.ID+"/receipt", strings.NewReader(""))
	recorder := httptest.NewRecorder()
	api.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("corrupt receipt escaped as client validation: status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"code":"internal_error"`) {
		t.Fatalf("expected internal_error response, body=%s", recorder.Body.String())
	}
}
