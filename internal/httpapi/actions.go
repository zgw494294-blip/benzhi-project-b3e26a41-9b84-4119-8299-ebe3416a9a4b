package httpapi

import (
	"net/http"

	"benzhi-project-b3e26a41-9b84-4119-8299-ebe3416a9a4b/internal/domain"
	"benzhi-project-b3e26a41-9b84-4119-8299-ebe3416a9a4b/internal/workflow"
)

func (a *API) submitAction(w http.ResponseWriter, r *http.Request, action func(workflow.SubmitCommand) (any, bool, error)) {
	var request writeRequest
	if decodeJSON(w, r, &request) != nil {
		return
	}
	meta, err := commandMeta(r, request)
	if err != nil {
		writeError(w, err)
		return
	}
	result, replayed, err := action(workflow.SubmitCommand{BatchID: r.PathValue("batchID"), Meta: meta})
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, http.StatusOK, result, replayed)
}

func (a *API) HandleSubmitReview(w http.ResponseWriter, r *http.Request) {
	a.submitAction(w, r, func(command workflow.SubmitCommand) (any, bool, error) {
		return a.service.SubmitReview(r.Context(), command)
	})
}

func (a *API) HandleCompleteReview(w http.ResponseWriter, r *http.Request) {
	a.submitAction(w, r, func(command workflow.SubmitCommand) (any, bool, error) {
		return a.service.CompleteReview(r.Context(), command)
	})
}

func (a *API) HandleFreezeManifest(w http.ResponseWriter, r *http.Request) {
	a.submitAction(w, r, func(command workflow.SubmitCommand) (any, bool, error) {
		return a.service.FreezeManifest(r.Context(), command)
	})
}

type reviewRequest struct {
	writeRequest
	Decision     domain.Decision `json:"decision"`
	ReasonCode   string          `json:"reasonCode"`
	Comment      string          `json:"comment"`
	ReviewerName string          `json:"reviewerName"`
}

func (a *API) HandleReviewItem(w http.ResponseWriter, r *http.Request) {
	var request reviewRequest
	if decodeJSON(w, r, &request) != nil {
		return
	}
	meta, err := commandMeta(r, request.writeRequest)
	if err != nil {
		writeError(w, err)
		return
	}
	batch, replayed, err := a.service.DecideReview(r.Context(), workflow.ReviewCommand{
		BatchID: r.PathValue("batchID"), ItemID: r.PathValue("itemID"), Meta: meta,
		Input: domain.ReviewInput{Decision: request.Decision, ReasonCode: request.ReasonCode, Comment: request.Comment, ReviewerName: request.ReviewerName},
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, http.StatusOK, batch, replayed)
}
