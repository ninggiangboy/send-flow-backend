package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	identityapp "github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
)

type settingsHTTP struct {
	svc *identityapp.Service
}

func newSettingsHTTP(svc *identityapp.Service) *settingsHTTP {
	return &settingsHTTP{svc: svc}
}

type settingsResponse struct {
	Version         int64                 `json:"version"`
	EmailDefaults   settingsEmailDefaults `json:"email_defaults"`
	FeatureControls map[string]any        `json:"feature_controls"`
	UpdatedByUserID string                `json:"updated_by_user_id,omitempty"`
	CreatedAt       time.Time             `json:"created_at"`
	UpdatedAt       time.Time             `json:"updated_at"`
}

type settingsEmailDefaults struct {
	DefaultSenderDomainID string `json:"default_sender_domain_id"`
}

func (h *settingsHTTP) getSettings(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	userID, _ := r.Context().Value(ctxUserID).(string)

	settings, err := h.svc.GetWorkspaceSettings(r.Context(), workspaceID, userID)
	if err != nil {
		writeSettingsErr(w, r, err)
		return
	}

	writeEnvelope(w, r, http.StatusOK, settingsToResponse(settings))
}

func (h *settingsHTTP) updateSettings(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	userID, _ := r.Context().Value(ctxUserID).(string)

	var req struct {
		Version         *int64                 `json:"version"`
		EmailDefaults   *settingsEmailDefaults `json:"email_defaults,omitempty"`
		FeatureControls *map[string]any        `json:"feature_controls,omitempty"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	patch := domain.WorkspaceSettingsPatch{
		Version: req.Version,
	}
	if req.EmailDefaults != nil {
		patch.EmailDefaults = &domain.EmailDefaults{
			DefaultSenderDomainID: req.EmailDefaults.DefaultSenderDomainID,
		}
	}
	if req.FeatureControls != nil {
		fc := domain.FeatureControls(*req.FeatureControls)
		patch.FeatureControls = &fc
	}

	if patch.EmailDefaults == nil && patch.FeatureControls == nil {
		writeError(w, r, http.StatusUnprocessableEntity, "settings.payload_invalid", "at least one setting field must be provided", nil)
		return
	}

	reqCtx := r.Context().Value(ctxRequestContext).(*requestLogContext)
	settings, err := h.svc.UpdateWorkspaceSettings(r.Context(), workspaceID, userID, reqCtx.RequestID, patch, time.Now().UTC())
	if err != nil {
		writeSettingsErr(w, r, err)
		return
	}

	writeEnvelope(w, r, http.StatusOK, settingsToResponse(settings))
}

func settingsToResponse(s *domain.WorkspaceSettings) settingsResponse {
	return settingsResponse{
		Version:         s.Version,
		EmailDefaults:   settingsEmailDefaults{DefaultSenderDomainID: s.EmailDefaults.DefaultSenderDomainID},
		FeatureControls: s.FeatureControls,
		UpdatedByUserID: s.UpdatedByUserID,
		CreatedAt:       s.CreatedAt,
		UpdatedAt:       s.UpdatedAt,
	}
}

func writeSettingsErr(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrSettingsManageDenied):
		writeError(w, r, http.StatusForbidden, "settings.manage_denied", err.Error(), nil)
	case errors.Is(err, domain.ErrSettingsVersionConflict):
		writeError(w, r, http.StatusConflict, "settings.version_conflict", err.Error(), nil)
	case errors.Is(err, domain.ErrSettingsPayloadInvalid):
		writeError(w, r, http.StatusUnprocessableEntity, "settings.payload_invalid", err.Error(), nil)
	case errors.Is(err, domain.ErrWorkspaceNotFound):
		writeError(w, r, http.StatusNotFound, "identity.workspace_not_found", err.Error(), nil)
	default:
		writeError(w, r, http.StatusInternalServerError, "internal.error", "internal error", nil)
	}
}
