package httpapi

import (
	"net/http"

	"benzhi-project-b3e26a41-9b84-4119-8299-ebe3416a9a4b/internal/domain"
	"benzhi-project-b3e26a41-9b84-4119-8299-ebe3416a9a4b/internal/workflow"
)

type correctionRequest struct {
	writeRequest
	MaterialName     string   `json:"materialName"`
	HazardClasses    []string `json:"hazardClasses"`
	Quantity         float64  `json:"quantity"`
	Unit             string   `json:"unit"`
	DisposalCategory string   `json:"disposalCategory"`
	ContainerType    string   `json:"containerType"`
	SealChecked      bool     `json:"sealChecked"`
	LabelChecked     bool     `json:"labelChecked"`
	Note             string   `json:"note"`
}

func (a *API) HandleCorrectItem(w http.ResponseWriter, r *http.Request) {
	var request correctionRequest
	if decodeJSON(w, r, &request) != nil {
		return
	}
	meta, err := commandMeta(r, request.writeRequest)
	if err != nil {
		writeError(w, err)
		return
	}
	batch, replayed, err := a.service.CorrectItem(r.Context(), workflow.CorrectionCommand{
		BatchID: r.PathValue("batchID"), ItemID: r.PathValue("itemID"), Meta: meta,
		Item:    domain.ItemInput{MaterialName: request.MaterialName, HazardClasses: request.HazardClasses, Quantity: request.Quantity, Unit: request.Unit, DisposalCategory: request.DisposalCategory},
		Package: domain.PackageInput{ContainerType: request.ContainerType, SealChecked: request.SealChecked, LabelChecked: request.LabelChecked},
		Note:    request.Note,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, http.StatusOK, batch, replayed)
}

type confirmationRequest struct {
	writeRequest
	Name string `json:"name"`
}

func (a *API) HandleConfirmation(w http.ResponseWriter, r *http.Request) {
	var request confirmationRequest
	if decodeJSON(w, r, &request) != nil {
		return
	}
	meta, err := commandMeta(r, request.writeRequest)
	if err != nil {
		writeError(w, err)
		return
	}
	batch, replayed, err := a.service.ConfirmHandover(r.Context(), workflow.ConfirmCommand{BatchID: r.PathValue("batchID"), Meta: meta, Name: request.Name})
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, http.StatusOK, batch, replayed)
}
