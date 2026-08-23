package httpapi

import (
	"net/http"
	"strconv"

	"benzhi-project-b3e26a41-9b84-4119-8299-ebe3416a9a4b/internal/domain"
)

func (a *API) HandleListBatches(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	due := domain.DueStatus(r.URL.Query().Get("dueStatus"))
	batches, err := a.service.ListBatches(r.Context(), limit, offset, due)
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, http.StatusOK, batches, false)
}

func (a *API) HandleReceiptVerification(w http.ResponseWriter, r *http.Request) {
	report, err := a.service.VerifyReceipt(r.Context(), r.PathValue("batchID"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, http.StatusOK, report, false)
}

func (a *API) HandleReceiptDownload(w http.ResponseWriter, r *http.Request) {
	bundle, err := a.service.ReceiptDownload(r.Context(), r.PathValue("batchID"))
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Content-Disposition", "attachment; filename=archive-receipt-"+r.PathValue("batchID")+".json")
	writeData(w, http.StatusOK, bundle, false)
}

func (a *API) HandleGetBatch(w http.ResponseWriter, r *http.Request) {
	role, ok := parseRoleOrWrite(w, r)
	if !ok {
		return
	}
	projection, err := a.service.Projection(r.Context(), r.PathValue("batchID"), role)
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, http.StatusOK, projection, false)
}

func (a *API) HandleTimeline(w http.ResponseWriter, r *http.Request) {
	timeline, err := a.service.Timeline(r.Context(), r.PathValue("batchID"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, http.StatusOK, timeline, false)
}

func (a *API) HandleReceipt(w http.ResponseWriter, r *http.Request) {
	receipt, err := a.service.GetReceipt(r.Context(), r.PathValue("batchID"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, http.StatusOK, receipt, false)
}
