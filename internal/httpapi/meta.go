package httpapi

import (
	"net/http"
	"strings"

	"benzhi-project-b3e26a41-9b84-4119-8299-ebe3416a9a4b/internal/domain"
	"benzhi-project-b3e26a41-9b84-4119-8299-ebe3416a9a4b/internal/workflow"
)

type writeRequest struct {
	RequestID       string `json:"requestId"`
	ExpectedVersion int64  `json:"expectedVersion"`
	Actor           string `json:"actor"`
}

func roleFromRequest(r *http.Request) (workflow.Role, error) {
	return workflow.ParseRole(r.Header.Get("X-Role"))
}

func commandMeta(r *http.Request, request writeRequest) (workflow.CommandMeta, error) {
	role, err := roleFromRequest(r)
	if err != nil {
		return workflow.CommandMeta{}, err
	}
	requestID := strings.TrimSpace(request.RequestID)
	if requestID == "" {
		requestID = strings.TrimSpace(r.Header.Get("X-Request-ID"))
	}
	if requestID == "" {
		return workflow.CommandMeta{}, domain.Required("requestId", "requestId")
	}
	return workflow.CommandMeta{
		RequestID: requestID, ExpectedVersion: request.ExpectedVersion,
		Role: role, Actor: strings.TrimSpace(request.Actor),
	}, nil
}

func parseRoleOrWrite(w http.ResponseWriter, r *http.Request) (workflow.Role, bool) {
	role, err := roleFromRequest(r)
	if err != nil {
		writeError(w, err)
		return "", false
	}
	return role, true
}
