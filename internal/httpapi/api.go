package httpapi

import (
	"io/fs"
	"log/slog"
	"net/http"

	"benzhi-project-b3e26a41-9b84-4119-8299-ebe3416a9a4b/internal/webui"
	"benzhi-project-b3e26a41-9b84-4119-8299-ebe3416a9a4b/internal/workflow"
)

const maxRequestBody = 1 << 20

type API struct {
	service *workflow.Service
	logger  *slog.Logger
	mux     *http.ServeMux
	assets  fs.FS
}

func New(service *workflow.Service, logger *slog.Logger) *API {
	if logger == nil {
		logger = slog.Default()
	}
	api := &API{service: service, logger: logger, mux: http.NewServeMux(), assets: webui.Assets()}
	api.routes()
	return api
}

func (a *API) Handler() http.Handler {
	return a.recoverPanic(a.accessLog(a.securityHeaders(a.mux)))
}

func (a *API) routes() {
	a.mux.HandleFunc("GET /", a.HandleIndex)
	a.mux.HandleFunc("GET /assets/styles.css", a.HandleStyles)
	a.mux.HandleFunc("GET /assets/app.js", a.HandleScript)
	a.mux.HandleFunc("GET /api/health", a.HandleHealth)
	a.mux.HandleFunc("GET /api/batches", a.HandleListBatches)
	a.mux.HandleFunc("POST /api/batches", a.HandleCreateBatch)
	a.mux.HandleFunc("GET /api/batches/{batchID}", a.HandleGetBatch)
	a.mux.HandleFunc("GET /api/batches/{batchID}/timeline", a.HandleTimeline)
	a.mux.HandleFunc("GET /api/batches/{batchID}/receipt", a.HandleReceipt)
	a.mux.HandleFunc("GET /api/batches/{batchID}/receipt/verification", a.HandleReceiptVerification)
	a.mux.HandleFunc("GET /api/batches/{batchID}/receipt/download", a.HandleReceiptDownload)
	a.mux.HandleFunc("POST /api/batches/{batchID}/reschedule", a.HandleReschedule)
	a.mux.HandleFunc("POST /api/batches/{batchID}/items", a.HandleAddItem)
	a.mux.HandleFunc("PUT /api/batches/{batchID}/items/{itemID}", a.HandleUpdateItem)
	a.mux.HandleFunc("PUT /api/batches/{batchID}/items/{itemID}/package", a.HandleSetPackage)
	a.mux.HandleFunc("POST /api/batches/{batchID}/packages/bulk", a.HandleSetPackagesBulk)
	a.mux.HandleFunc("POST /api/batches/{batchID}/submit-review", a.HandleSubmitReview)
	a.mux.HandleFunc("POST /api/batches/{batchID}/reviews/{itemID}", a.HandleReviewItem)
	a.mux.HandleFunc("POST /api/batches/{batchID}/reviews/bulk", a.HandleReviewBulk)
	a.mux.HandleFunc("POST /api/batches/{batchID}/complete-review", a.HandleCompleteReview)
	a.mux.HandleFunc("POST /api/batches/{batchID}/corrections/{itemID}", a.HandleCorrectItem)
	a.mux.HandleFunc("POST /api/batches/{batchID}/freeze", a.HandleFreezeManifest)
	a.mux.HandleFunc("POST /api/batches/{batchID}/confirmations", a.HandleConfirmation)
}
