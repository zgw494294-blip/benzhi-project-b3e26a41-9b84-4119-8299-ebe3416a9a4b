package concurrent_request_buffer_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"benzhi-project-b3e26a41-9b84-4119-8299-ebe3416a9a4b/internal/httpapi"
	"benzhi-project-b3e26a41-9b84-4119-8299-ebe3416a9a4b/internal/storage"
	"benzhi-project-b3e26a41-9b84-4119-8299-ebe3416a9a4b/internal/workflow"
)

type gatedBody struct {
	data     []byte
	offset   int
	copied   chan struct{}
	release  <-chan struct{}
	notified bool
}

func (b *gatedBody) Read(p []byte) (int, error) {
	if b.offset < len(b.data) {
		n := copy(p, b.data[b.offset:])
		b.offset += n
		return n, nil
	}
	if !b.notified {
		close(b.copied)
		b.notified = true
	}
	<-b.release
	return 0, io.EOF
}

func (b *gatedBody) Close() error { return nil }

func TestConcurrentRequestBufferIsolation(t *testing.T) {
	store, err := storage.Open(context.Background(), storage.Options{Path: ":memory:"})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	now := time.Date(2026, time.August, 24, 9, 0, 0, 0, time.UTC)
	var sequence atomic.Int64
	service := workflow.New(store, workflow.Options{
		Clock:       func() time.Time { return now },
		IDGenerator: func(prefix string) string { return prefix + "-" + strconv.FormatInt(sequence.Add(1), 10) },
	})
	handler := httpapi.New(service, nil).Handler()

	bodyA := `{"requestId":"request-a","sourceLab":"甲实验室","ownerName":"甲安全员","plannedHandoverAt":"2026-08-25T09:00:00Z"}`
	bodyB := `{"requestId":"request-b","sourceLab":"乙实验室","ownerName":"乙安全员","plannedHandoverAt":"2026-08-26T09:00:00Z"}`
	copied := make(chan struct{})
	release := make(chan struct{})
	requestA := httptest.NewRequest(http.MethodPost, "/api/batches", &gatedBody{
		data: []byte(bodyA), copied: copied, release: release,
	})
	requestA.Header.Set("Content-Type", "application/json")
	requestA.Header.Set("X-Role", "safety_officer")
	responseA := httptest.NewRecorder()
	doneA := make(chan struct{})
	go func() {
		handler.ServeHTTP(responseA, requestA)
		close(doneA)
	}()

	<-copied
	requestB := httptest.NewRequest(http.MethodPost, "/api/batches", strings.NewReader(bodyB))
	requestB.Header.Set("Content-Type", "application/json")
	requestB.Header.Set("X-Role", "safety_officer")
	responseB := httptest.NewRecorder()
	handler.ServeHTTP(responseB, requestB)
	close(release)
	<-doneA

	if responseB.Code != http.StatusCreated {
		t.Fatalf("second request failed: status=%d body=%s", responseB.Code, responseB.Body.String())
	}
	var result struct {
		Data struct {
			SourceLab string `json:"sourceLab"`
		} `json:"data"`
	}
	if err := json.Unmarshal(responseA.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode first response: %v", err)
	}
	if responseA.Code != http.StatusCreated || result.Data.SourceLab != "甲实验室" {
		t.Fatalf("concurrent decode reused request body: status=%d sourceLab=%q body=%s", responseA.Code, result.Data.SourceLab, responseA.Body.String())
	}
}
