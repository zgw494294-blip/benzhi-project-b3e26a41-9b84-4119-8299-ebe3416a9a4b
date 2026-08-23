package httpapi

import (
	"net/http"
	"strings"

	"benzhi-project-b3e26a41-9b84-4119-8299-ebe3416a9a4b/internal/domain"
	"benzhi-project-b3e26a41-9b84-4119-8299-ebe3416a9a4b/internal/workflow"
)

type createBatchRequest struct {
	RequestID         string `json:"requestId"`
	SourceLab         string `json:"sourceLab"`
	OwnerName         string `json:"ownerName"`
	PlannedHandoverAt string `json:"plannedHandoverAt"`
}

func (a *API) HandleCreateBatch(w http.ResponseWriter, r *http.Request) {
	var request createBatchRequest
	if decodeJSON(w, r, &request) != nil {
		return
	}
	role, err := roleFromRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	planned, err := parseTime(request.PlannedHandoverAt)
	if err != nil {
		writeError(w, domain.Invalid("plannedHandoverAt", "计划交接日期必须是 RFC3339 时间"))
		return
	}
	requestID := strings.TrimSpace(request.RequestID)
	if requestID == "" {
		requestID = strings.TrimSpace(r.Header.Get("X-Request-ID"))
	}
	batch, replayed, err := a.service.CreateBatch(r.Context(), workflow.CreateBatchCommand{
		RequestID: requestID, Role: role,
		Input: domain.BatchInput{SourceLab: request.SourceLab, OwnerName: request.OwnerName, PlannedHandoverAt: planned},
	})
	if err != nil {
		writeError(w, err)
		return
	}
	status := http.StatusCreated
	if replayed {
		status = http.StatusOK
	}
	writeData(w, status, batch, replayed)
}
