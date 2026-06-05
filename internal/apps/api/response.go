package api

import (
	"net/http"
	"strings"

	"github.com/ninggiangboy/send-flow/backend/internal/platform/httpjson"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/id"
)

type meta struct {
	RequestID string `json:"request_id"`
}

type envelope struct {
	Data any  `json:"data"`
	Meta meta `json:"meta"`
}

type errorBody struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

type errorEnvelope struct {
	Error errorBody `json:"error"`
	Meta  meta      `json:"meta"`
}

func requestID(r *http.Request) string {
	if reqCtx, ok := r.Context().Value(ctxRequestContext).(*requestLogContext); ok && strings.TrimSpace(reqCtx.RequestID) != "" {
		return reqCtx.RequestID
	}
	if rid := strings.TrimSpace(r.Header.Get("X-Request-Id")); rid != "" {
		return rid
	}
	return id.Must(id.NewUUIDGenerator())
}

func writeEnvelope(w http.ResponseWriter, r *http.Request, status int, data any) {
	httpjson.Write(w, status, envelope{Data: data, Meta: meta{RequestID: requestID(r)}})
}

func writeError(w http.ResponseWriter, r *http.Request, status int, code, message string, details map[string]any) {
	httpjson.Write(w, status, errorEnvelope{Error: errorBody{Code: code, Message: message, Details: details}, Meta: meta{RequestID: requestID(r)}})
}
