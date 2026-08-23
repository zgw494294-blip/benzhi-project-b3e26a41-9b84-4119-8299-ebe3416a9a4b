package httpapi

import (
	"net/http"
	"strings"

	"benzhi-project-b3e26a41-9b84-4119-8299-ebe3416a9a4b/internal/domain"
	"benzhi-project-b3e26a41-9b84-4119-8299-ebe3416a9a4b/internal/workflow"
)

type rescheduleRequest struct {
	writeRequest
	PlannedHandoverAt string `json:"plannedHandoverAt"`
	Reason            string `json:"reason"`
}

func (a *API) HandleReschedule(w http.ResponseWriter, r *http.Request) {
	var request rescheduleRequest
	if decodeJSON(w, r, &request) != nil {
		return
	}
	meta, err := commandMeta(r, request.writeRequest)
	if err != nil {
		writeError(w, err)
		return
	}
	planned, err := parseTime(request.PlannedHandoverAt)
	if err != nil {
		writeError(w, domain.Invalid("plannedHandoverAt", "计划交接日期必须是 RFC3339 时间"))
		return
	}
	batch, replayed, err := a.service.Reschedule(r.Context(), workflow.RescheduleCommand{BatchID: r.PathValue("batchID"), Meta: meta, PlannedHandoverAt: planned, Reason: request.Reason})
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, http.StatusOK, batch, replayed)
}

type packageBulkItem struct {
	ItemID        string `json:"itemId"`
	ContainerType string `json:"containerType"`
	SealChecked   bool   `json:"sealChecked"`
	LabelChecked  bool   `json:"labelChecked"`
}
type packageBulkRequest struct {
	writeRequest
	Items []packageBulkItem `json:"items"`
}

func (a *API) HandleSetPackagesBulk(w http.ResponseWriter, r *http.Request) {
	var request packageBulkRequest
	if decodeJSON(w, r, &request) != nil {
		return
	}
	meta, err := commandMeta(r, request.writeRequest)
	if err != nil {
		writeError(w, err)
		return
	}
	changes := make([]domain.PackageChange, len(request.Items))
	for index, item := range request.Items {
		changes[index] = domain.PackageChange{ItemID: strings.TrimSpace(item.ItemID), Input: domain.PackageInput{ContainerType: item.ContainerType, SealChecked: item.SealChecked, LabelChecked: item.LabelChecked}}
	}
	batch, replayed, err := a.service.SetPackagesBulk(r.Context(), workflow.PackageBulkCommand{BatchID: r.PathValue("batchID"), Meta: meta, Changes: changes})
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, http.StatusOK, batch, replayed)
}

type reviewBulkItem struct {
	ItemID       string          `json:"itemId"`
	Decision     domain.Decision `json:"decision"`
	ReasonCode   string          `json:"reasonCode"`
	Comment      string          `json:"comment"`
	ReviewerName string          `json:"reviewerName"`
}
type reviewBulkRequest struct {
	writeRequest
	Items []reviewBulkItem `json:"items"`
}

func (a *API) HandleReviewBulk(w http.ResponseWriter, r *http.Request) {
	var request reviewBulkRequest
	if decodeJSON(w, r, &request) != nil {
		return
	}
	meta, err := commandMeta(r, request.writeRequest)
	if err != nil {
		writeError(w, err)
		return
	}
	changes := make([]domain.ReviewChange, len(request.Items))
	for index, item := range request.Items {
		name := strings.TrimSpace(item.ReviewerName)
		if name == "" {
			name = meta.Actor
		}
		changes[index] = domain.ReviewChange{ItemID: strings.TrimSpace(item.ItemID), Input: domain.ReviewInput{Decision: item.Decision, ReasonCode: item.ReasonCode, Comment: item.Comment, ReviewerName: name}}
	}
	batch, replayed, err := a.service.DecideReviewsBulk(r.Context(), workflow.ReviewBulkCommand{BatchID: r.PathValue("batchID"), Meta: meta, Changes: changes})
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, http.StatusOK, batch, replayed)
}
