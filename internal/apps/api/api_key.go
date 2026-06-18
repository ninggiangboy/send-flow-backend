package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	accessapp "github.com/ninggiangboy/send-flow/backend/internal/modules/access/app"
	accessdomain "github.com/ninggiangboy/send-flow/backend/internal/modules/access/domain"
	identityapp "github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/auth"
)

type apiKeyHTTP struct {
	svc           *accessapp.Service
	auditRecorder identityapp.AuditRecorder
}

func newAPIKeyHTTP(svc *accessapp.Service, auditRecorder identityapp.AuditRecorder) *apiKeyHTTP {
	return &apiKeyHTTP{svc: svc, auditRecorder: auditRecorder}
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
		Name        string                         `json:"name"`
		Scopes      []string                       `json:"scopes"`
		ExpiresAt   *time.Time                     `json:"expires_at"`
		QuotaLimits *accessdomain.EmailQuotaLimits `json:"email_quota_limits,omitempty"`
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
		QuotaLimits: req.QuotaLimits,
	})
	if err != nil {
		writeAPIKeyErr(w, r, err)
		return
	}

	h.recordAudit(r, identityapp.RecordAuditInput{
		WorkspaceID: workspaceID,
		ActorUserID: userID,
		ActionType:  "api_key.created",
		TargetType:  "api_key",
		TargetID:    result.ID,
		PayloadSummary: map[string]any{
			"name":               result.Name,
			"scopes":             result.Scopes,
			"expires_at":         result.ExpiresAt,
			"email_quota_limits": result.QuotaLimits,
		},
	})

	writeEnvelope(w, r, http.StatusCreated, map[string]any{
		"id":                 result.ID,
		"name":               result.Name,
		"key_prefix":         result.KeyPrefix,
		"scopes":             result.Scopes,
		"status":             result.Status,
		"secret":             result.Secret,
		"created_at":         result.CreatedAt,
		"expires_at":         result.ExpiresAt,
		"email_quota_limits": result.QuotaLimits,
	})
}

func (h *apiKeyHTTP) updateAPIKey(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	apiKeyID := chi.URLParam(r, "api_key_id")
	userID, _ := r.Context().Value(ctxUserID).(string)

	var req struct {
		Name        *string          `json:"name"`
		Scopes      []string         `json:"scopes"`
		ExpiresAt   *time.Time       `json:"expires_at"`
		Rotate      bool             `json:"rotate"`
		QuotaLimits *json.RawMessage `json:"email_quota_limits"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	// Resolve patch semantics for quota limits:
	//   nil            → omitted from JSON → pass nil (no change)
	//   "null"         → explicit null     → pass empty struct (clear)
	//   {...}          → provided object   → unmarshal and pass (replace)
	var quotaLimits *accessdomain.EmailQuotaLimits
	if req.QuotaLimits != nil {
		if string(*req.QuotaLimits) == "null" {
			quotaLimits = &accessdomain.EmailQuotaLimits{}
		} else {
			var ql accessdomain.EmailQuotaLimits
			if err := json.Unmarshal(*req.QuotaLimits, &ql); err != nil {
				writeAPIKeyErr(w, r, accessdomain.ErrAPIKeyConfigInvalid)
				return
			}
			quotaLimits = &ql
		}
	}

	result, err := h.svc.UpdateAPIKey(r.Context(), accessapp.UpdateAPIKeyInput{
		WorkspaceID: workspaceID,
		ActorUserID: userID,
		APIKeyID:    apiKeyID,
		Name:        req.Name,
		Scopes:      req.Scopes,
		ExpiresAt:   req.ExpiresAt,
		Rotate:      req.Rotate,
		QuotaLimits: quotaLimits,
	})
	if err != nil {
		writeAPIKeyErr(w, r, err)
		return
	}

	actionType := "api_key.updated"
	if req.Rotate {
		actionType = "api_key.rotated"
	}
	h.recordAudit(r, identityapp.RecordAuditInput{
		WorkspaceID: workspaceID,
		ActorUserID: userID,
		ActionType:  actionType,
		TargetType:  "api_key",
		TargetID:    apiKeyID,
		PayloadSummary: map[string]any{
			"email_quota_limits": result.QuotaLimits,
		},
	})

	resp := map[string]any{
		"id":                 result.ID,
		"name":               result.Name,
		"key_prefix":         result.KeyPrefix,
		"scopes":             result.Scopes,
		"status":             result.Status,
		"created_at":         result.CreatedAt,
		"updated_at":         result.UpdatedAt,
		"expires_at":         result.ExpiresAt,
		"email_quota_limits": result.QuotaLimits,
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

	h.recordAudit(r, identityapp.RecordAuditInput{
		WorkspaceID:    workspaceID,
		ActorUserID:    userID,
		ActionType:     "api_key.revoked",
		TargetType:     "api_key",
		TargetID:       apiKeyID,
		PayloadSummary: map[string]any{},
	})

	w.WriteHeader(http.StatusNoContent)
}

func (h *apiKeyHTTP) recordAudit(r *http.Request, input identityapp.RecordAuditInput) {
	if h.auditRecorder == nil {
		return
	}
	reqCtx := r.Context().Value(ctxRequestContext).(*requestLogContext)
	input.RequestID = reqCtx.RequestID
	input.OccurredAt = time.Now().UTC()
	if err := h.auditRecorder.Record(r.Context(), input); err != nil {
		slog.Warn("failed to record audit event", "error", err)
	}
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
	case errors.Is(err, accessdomain.ErrEmailQuotaInvalid):
		writeError(w, r, http.StatusUnprocessableEntity, "api_key.config_invalid", err.Error(), nil)
	case errors.Is(err, accessdomain.ErrPayloadInvalid):
		writeError(w, r, http.StatusBadRequest, "auth.invalid_request_body", err.Error(), nil)
	case errors.Is(err, auth.ErrPermissionDenied):
		writeError(w, r, http.StatusForbidden, "auth.permission_denied", err.Error(), nil)
	default:
		writeError(w, r, http.StatusInternalServerError, "internal.error", "internal error", nil)
	}
}
