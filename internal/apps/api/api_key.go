package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	accessapp "github.com/ninggiangboy/send-flow/backend/internal/modules/access/app"
	accessdomain "github.com/ninggiangboy/send-flow/backend/internal/modules/access/domain"
)

type apiKeyHTTP struct {
	svc *accessapp.Service
}

func newAPIKeyHTTP(svc *accessapp.Service) *apiKeyHTTP {
	return &apiKeyHTTP{svc: svc}
}

func (h *apiKeyHTTP) listAPIKeys(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	userID, _ := r.Context().Value(ctxUserID).(string)

	limit := 50
	cursor := r.URL.Query().Get("cursor")
	status := r.URL.Query().Get("status")

	result, err := h.svc.ListAPIKeys(r.Context(), accessapp.ListAPIKeysInput{
		WorkspaceID: workspaceID,
		ActorUserID: userID,
		Status:      status,
		Limit:       limit,
		Cursor:      cursor,
	})
	if err != nil {
		writeAPIKeyErr(w, r, err)
		return
	}

	resp := map[string]any{
		"data":   result.APIKeys,
		"cursor": result.Cursor,
	}
	writeEnvelope(w, r, http.StatusOK, resp)
}

func (h *apiKeyHTTP) createAPIKey(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	userID, _ := r.Context().Value(ctxUserID).(string)

	var req struct {
		Name      string     `json:"name"`
		Scopes    []string   `json:"scopes"`
		ExpiresAt *time.Time `json:"expires_at"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	result, err := h.svc.CreateAPIKey(r.Context(), accessapp.CreateAPIKeyInput{
		WorkspaceID: workspaceID,
		ActorUserID: userID,
		Name:        req.Name,
		Scopes:      req.Scopes,
		ExpiresAt:   req.ExpiresAt,
	})
	if err != nil {
		writeAPIKeyErr(w, r, err)
		return
	}

	writeEnvelope(w, r, http.StatusCreated, map[string]any{
		"id":         result.ID,
		"name":       result.Name,
		"key_prefix": result.KeyPrefix,
		"scopes":     result.Scopes,
		"status":     result.Status,
		"secret":     result.Secret,
		"created_at": result.CreatedAt,
		"expires_at": result.ExpiresAt,
	})
}

func (h *apiKeyHTTP) updateAPIKey(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	apiKeyID := chi.URLParam(r, "api_key_id")
	userID, _ := r.Context().Value(ctxUserID).(string)

	var req struct {
		Name      *string    `json:"name"`
		Scopes    []string   `json:"scopes"`
		ExpiresAt *time.Time `json:"expires_at"`
		Rotate    bool       `json:"rotate"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	result, err := h.svc.UpdateAPIKey(r.Context(), accessapp.UpdateAPIKeyInput{
		WorkspaceID: workspaceID,
		ActorUserID: userID,
		APIKeyID:    apiKeyID,
		Name:        req.Name,
		Scopes:      req.Scopes,
		ExpiresAt:   req.ExpiresAt,
		Rotate:      req.Rotate,
	})
	if err != nil {
		writeAPIKeyErr(w, r, err)
		return
	}

	resp := map[string]any{
		"id":         result.ID,
		"name":       result.Name,
		"key_prefix": result.KeyPrefix,
		"scopes":     result.Scopes,
		"status":     result.Status,
		"created_at": result.CreatedAt,
		"updated_at": result.UpdatedAt,
		"expires_at": result.ExpiresAt,
	}
	if result.Secret != "" {
		resp["secret"] = result.Secret
	}
	writeEnvelope(w, r, http.StatusOK, resp)
}

func (h *apiKeyHTTP) revokeAPIKey(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	apiKeyID := chi.URLParam(r, "api_key_id")
	userID, _ := r.Context().Value(ctxUserID).(string)

	_, err := h.svc.RevokeAPIKey(r.Context(), accessapp.RevokeAPIKeyInput{
		WorkspaceID: workspaceID,
		ActorUserID: userID,
		APIKeyID:    apiKeyID,
	})
	if err != nil {
		writeAPIKeyErr(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func writeAPIKeyErr(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, accessdomain.ErrAPIKeyManageDenied):
		writeError(w, r, http.StatusForbidden, "api_key.manage_denied", err.Error(), nil)
	case errors.Is(err, accessdomain.ErrAPIKeyNotFound):
		writeError(w, r, http.StatusNotFound, "api_key.not_found", err.Error(), nil)
	case errors.Is(err, accessdomain.ErrAPIKeyRotateConflict):
		writeError(w, r, http.StatusConflict, "api_key.rotate_conflict", err.Error(), nil)
	case errors.Is(err, accessdomain.ErrAPIKeyScopeInvalid):
		writeError(w, r, http.StatusUnprocessableEntity, "api_key.scope_invalid", err.Error(), nil)
	case errors.Is(err, accessdomain.ErrAPIKeyConfigInvalid):
		writeError(w, r, http.StatusUnprocessableEntity, "api_key.config_invalid", err.Error(), nil)
	case errors.Is(err, accessdomain.ErrPayloadInvalid):
		writeError(w, r, http.StatusBadRequest, "auth.invalid_request_body", err.Error(), nil)
	default:
		writeError(w, r, http.StatusInternalServerError, "health.runtime_not_ready", "internal error", nil)
	}
}
