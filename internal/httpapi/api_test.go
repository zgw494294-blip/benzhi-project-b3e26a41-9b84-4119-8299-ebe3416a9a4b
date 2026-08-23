package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"benzhi-project-b3e26a41-9b84-4119-8299-ebe3416a9a4b/internal/storage"
	"benzhi-project-b3e26a41-9b84-4119-8299-ebe3416a9a4b/internal/workflow"
)

func TestAPIPageHealthAndProtocol(t *testing.T) {
	store, err := storage.Open(context.Background(), storage.Options{Path: ":memory:"})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	api := New(workflow.New(store, workflow.Options{}), nil)
	server := httptest.NewServer(api.Handler())
	defer server.Close()
	page, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	if page.StatusCode != http.StatusOK {
		t.Fatalf("page status %d", page.StatusCode)
	}
	page.Body.Close()
	health, err := http.Get(server.URL + "/api/health")
	if err != nil {
		t.Fatal(err)
	}
	if health.StatusCode != http.StatusOK || !strings.Contains(health.Header.Get("Content-Type"), "application/json") {
		t.Fatalf("health response status=%d type=%s", health.StatusCode, health.Header.Get("Content-Type"))
	}
	health.Body.Close()
	request, err := http.NewRequest(http.MethodPost, server.URL+"/api/batches", strings.NewReader(`{"requestId":"bad"}`))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("X-Role", "safety_officer")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusUnsupportedMediaType {
		t.Fatalf("expected media type error, got %d", response.StatusCode)
	}
	response.Body.Close()
}

func TestAPICreateBatch(t *testing.T) {
	store, err := storage.Open(context.Background(), storage.Options{Path: ":memory:"})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	api := New(workflow.New(store, workflow.Options{}), nil)
	requestBody := `{"requestId":"api-create","sourceLab":"API 实验室","ownerName":"王安全","plannedHandoverAt":"` + time.Now().UTC().Add(24*time.Hour).Format(time.RFC3339) + `"}`
	request := httptest.NewRequest(http.MethodPost, "/api/batches", strings.NewReader(requestBody))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Role", "safety_officer")
	recorder := httptest.NewRecorder()
	api.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Data struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Data.ID == "" || response.Data.Status != "draft" {
		t.Fatalf("unexpected create response: %#v", response)
	}
}
