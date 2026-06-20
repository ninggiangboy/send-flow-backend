package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	platformconstants "github.com/ninggiangboy/send-flow/backend/internal/platform/constants"
)

func pathParam(r *http.Request, key string) string {
	return chi.URLParam(r, key)
}

func workspaceIDParam(r *http.Request) string {
	return pathParam(r, "workspace_id")
}

func messageIDParam(r *http.Request) string {
	return pathParam(r, "message_id")
}

func campaignIDParam(r *http.Request) string {
	return pathParam(r, "campaign_id")
}

func requestIDParam(r *http.Request) string {
	return pathParam(r, "request_id")
}

func providerParam(r *http.Request) string {
	return pathParam(r, "provider")
}

func providerEventIDParam(r *http.Request) string {
	return pathParam(r, "provider_event_id")
}

func tokenParam(r *http.Request) string {
	return pathParam(r, "token")
}

func sessionIDParam(r *http.Request) string {
	return pathParam(r, "session_id")
}

func headerAuthorization(r *http.Request) string {
	return r.Header.Get(platformconstants.HeaderAuthorization)
}

func headerContentType(r *http.Request) string {
	return r.Header.Get(platformconstants.HeaderContentType)
}
