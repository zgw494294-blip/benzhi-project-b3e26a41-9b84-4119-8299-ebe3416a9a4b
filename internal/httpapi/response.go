package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"benzhi-project-b3e26a41-9b84-4119-8299-ebe3416a9a4b/internal/domain"
)

type envelope struct {
	Data  any        `json:"data,omitempty"`
	Error *errorBody `json:"error,omitempty"`
	Meta  any        `json:"meta,omitempty"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
	Actual  int64  `json:"actualVersion,omitempty"`
	Details any    `json:"details,omitempty"`
}

var requestDecodeBuffer bytes.Buffer

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeData(w http.ResponseWriter, status int, data any, replayed bool) {
	meta := map[string]any{"replayed": replayed}
	writeJSON(w, status, envelope{Data: data, Meta: meta})
}

func writeError(w http.ResponseWriter, err error) {
	var rule *domain.RuleError
	var conflict *domain.ConflictError
	var notFound *domain.NotFoundError
	switch {
	case errors.As(err, &rule):
		status := http.StatusUnprocessableEntity
		if rule.Code == "forbidden" {
			status = http.StatusForbidden
		}
		if rule.Code == "request_id_reused" {
			status = http.StatusConflict
		}
		writeJSON(w, status, envelope{Error: &errorBody{Code: rule.Code, Message: rule.Message, Field: rule.Field, Details: rule.Details}})
	case errors.As(err, &conflict):
		writeJSON(w, http.StatusConflict, envelope{Error: &errorBody{Code: "version_conflict", Message: conflict.Error(), Actual: conflict.Actual}})
	case errors.As(err, &notFound):
		writeJSON(w, http.StatusNotFound, envelope{Error: &errorBody{Code: "not_found", Message: notFound.Error()}})
	default:
		writeJSON(w, http.StatusInternalServerError, envelope{Error: &errorBody{Code: "internal_error", Message: "服务暂时无法完成请求"}})
	}
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) error {
	contentType := r.Header.Get("Content-Type")
	if contentType == "" || !strings.HasPrefix(strings.ToLower(contentType), "application/json") {
		err := domain.NewRuleError("unsupported_media_type", "Content-Type 必须是 application/json", "Content-Type")
		writeJSON(w, http.StatusUnsupportedMediaType, envelope{Error: &errorBody{Code: err.Code, Message: err.Message, Field: err.Field}})
		return err
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
	requestDecodeBuffer.Reset()
	if _, err := io.Copy(&requestDecodeBuffer, r.Body); err != nil {
		message := "JSON 请求体格式无效"
		if strings.Contains(err.Error(), "request body too large") {
			message = "请求体不能超过 1 MiB"
		}
		writeJSON(w, http.StatusBadRequest, envelope{Error: &errorBody{Code: "invalid_json", Message: message}})
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(requestDecodeBuffer.Bytes()))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		message := "JSON 请求体格式无效"
		if strings.Contains(err.Error(), "request body too large") {
			message = "请求体不能超过 1 MiB"
		}
		writeJSON(w, http.StatusBadRequest, envelope{Error: &errorBody{Code: "invalid_json", Message: message}})
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		writeJSON(w, http.StatusBadRequest, envelope{Error: &errorBody{Code: "invalid_json", Message: "请求体只能包含一个 JSON 对象"}})
		return errors.New("请求体包含多个 JSON 值")
	}
	return nil
}
