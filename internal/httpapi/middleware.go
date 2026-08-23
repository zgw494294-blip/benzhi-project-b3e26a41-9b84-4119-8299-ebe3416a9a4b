package httpapi

import (
	"encoding/json"
	"net/http"
	"runtime/debug"
	"time"
)

type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (r *statusRecorder) WriteHeader(status int) {
	if r.status == 0 {
		r.status = status
	}
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(data []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	count, err := r.ResponseWriter.Write(data)
	r.bytes += count
	return count, err
}

func (a *API) accessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		recorder := &statusRecorder{ResponseWriter: w}
		next.ServeHTTP(recorder, r)
		a.logger.Info("http_access",
			"method", r.Method, "path", r.URL.Path, "status", recorder.status,
			"bytes", recorder.bytes, "durationMs", time.Since(start).Milliseconds(),
			"requestId", requestIDForLog(r), "remote", r.RemoteAddr,
		)
	})
}

func requestIDForLog(r *http.Request) string {
	if value := r.Header.Get("X-Request-ID"); value != "" {
		return value
	}
	return "-"
}

func (a *API) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; connect-src 'self'")
		next.ServeHTTP(w, r)
	})
}

func (a *API) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				payload, _ := json.Marshal(map[string]any{"panic": recovered, "stack": string(debug.Stack())})
				a.logger.Error("http_panic", "detail", string(payload))
				writeJSON(w, http.StatusInternalServerError, envelope{Error: &errorBody{Code: "internal_error", Message: "服务暂时无法完成请求"}})
			}
		}()
		next.ServeHTTP(w, r)
	})
}
