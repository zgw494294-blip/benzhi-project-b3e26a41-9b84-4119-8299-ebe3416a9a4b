package httpapi

import (
	"net/http"

	"benzhi-project-b3e26a41-9b84-4119-8299-ebe3416a9a4b/internal/domain"
	"benzhi-project-b3e26a41-9b84-4119-8299-ebe3416a9a4b/internal/workflow"
)

type itemRequest struct {
	writeRequest
	MaterialName     string   `json:"materialName"`
	HazardClasses    []string `json:"hazardClasses"`
	Quantity         float64  `json:"quantity"`
	Unit             string   `json:"unit"`
	DisposalCategory string   `json:"disposalCategory"`
}

func (request itemRequest) input() domain.ItemInput {
	return domain.ItemInput{
		MaterialName: request.MaterialName, HazardClasses: request.HazardClasses,
		Quantity: request.Quantity, Unit: request.Unit, DisposalCategory: request.DisposalCategory,
	}
}

func (a *API) HandleAddItem(w http.ResponseWriter, r *http.Request) {
	var request itemRequest
	if decodeJSON(w, r, &request) != nil {
		return
	}
	meta, err := commandMeta(r, request.writeRequest)
	if err != nil {
		writeError(w, err)
		return
	}
	batch, replayed, err := a.service.AddItem(r.Context(), workflow.AddItemCommand{BatchID: r.PathValue("batchID"), Meta: meta, Input: request.input()})
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, http.StatusOK, batch, replayed)
}

func (a *API) HandleUpdateItem(w http.ResponseWriter, r *http.Request) {
	var request itemRequest
	if decodeJSON(w, r, &request) != nil {
		return
	}
	meta, err := commandMeta(r, request.writeRequest)
	if err != nil {
		writeError(w, err)
		return
	}
	batch, replayed, err := a.service.UpdateItem(r.Context(), workflow.UpdateItemCommand{BatchID: r.PathValue("batchID"), ItemID: r.PathValue("itemID"), Meta: meta, Input: request.input()})
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, http.StatusOK, batch, replayed)
}

type packageRequest struct {
	writeRequest
	ContainerType string `json:"containerType"`
	SealChecked   bool   `json:"sealChecked"`
	LabelChecked  bool   `json:"labelChecked"`
}

func (a *API) HandleSetPackage(w http.ResponseWriter, r *http.Request) {
	var request packageRequest
	if decodeJSON(w, r, &request) != nil {
		return
	}
	meta, err := commandMeta(r, request.writeRequest)
	if err != nil {
		writeError(w, err)
		return
	}
	batch, replayed, err := a.service.SetPackage(r.Context(), workflow.PackageCommand{
		BatchID: r.PathValue("batchID"), ItemID: r.PathValue("itemID"), Meta: meta,
		Input: domain.PackageInput{ContainerType: request.ContainerType, SealChecked: request.SealChecked, LabelChecked: request.LabelChecked},
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeData(w, http.StatusOK, batch, replayed)
}
