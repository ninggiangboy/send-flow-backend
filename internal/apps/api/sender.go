package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	senderapp "github.com/ninggiangboy/send-flow/backend/internal/modules/sender/app"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/sender/domain"
)

type senderHTTP struct {
	svc *senderapp.Service
}

func newSenderHTTP(svc *senderapp.Service) *senderHTTP {
	return &senderHTTP{svc: svc}
}

func (h *senderHTTP) listSenderDomains(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	userID, _ := r.Context().Value(ctxUserID).(string)

	results, err := h.svc.ListSenderDomains(r.Context(), workspaceID, userID)
	if err != nil {
		writeSenderErr(w, r, err)
		return
	}

	out := make([]map[string]any, 0, len(results))
	for _, res := range results {
		out = append(out, senderDomainResponse(res))
	}
	writeEnvelope(w, r, http.StatusOK, out)
}

func (h *senderHTTP) createSenderDomain(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	userID, _ := r.Context().Value(ctxUserID).(string)

	var req struct {
		Domain   string `json:"domain"`
		Provider string `json:"provider,omitempty"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	result, err := h.svc.CreateSenderDomain(r.Context(), workspaceID, userID, req.Domain, req.Provider, time.Now().UTC())
	if err != nil {
		writeSenderErr(w, r, err)
		return
	}
	writeEnvelope(w, r, http.StatusCreated, senderDomainResponse(*result))
}

func (h *senderHTTP) getSenderDomain(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	domainID := chi.URLParam(r, "domain_id")
	userID, _ := r.Context().Value(ctxUserID).(string)

	result, err := h.svc.GetSenderDomain(r.Context(), workspaceID, domainID, userID)
	if err != nil {
		writeSenderErr(w, r, err)
		return
	}
	writeEnvelope(w, r, http.StatusOK, senderDomainResponse(*result))
}

func (h *senderHTTP) verifySenderDomain(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	domainID := chi.URLParam(r, "domain_id")
	userID, _ := r.Context().Value(ctxUserID).(string)

	result, err := h.svc.RefreshSenderDomainDNSStatus(r.Context(), workspaceID, domainID, userID, time.Now().UTC())
	if err != nil {
		writeSenderErr(w, r, err)
		return
	}
	writeEnvelope(w, r, http.StatusOK, senderDomainResponse(*result))
}

func (h *senderHTTP) disableSenderDomain(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	domainID := chi.URLParam(r, "domain_id")
	userID, _ := r.Context().Value(ctxUserID).(string)

	result, err := h.svc.DisableSenderDomain(r.Context(), workspaceID, domainID, userID, time.Now().UTC())
	if err != nil {
		writeSenderErr(w, r, err)
		return
	}
	writeEnvelope(w, r, http.StatusOK, senderDomainResponse(*result))
}

func writeSenderErr(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrManageDenied):
		writeError(w, r, http.StatusForbidden, "sender.manage_denied", err.Error(), nil)
	case errors.Is(err, domain.ErrDomainNotFound):
		writeError(w, r, http.StatusNotFound, "sender.domain_not_found", err.Error(), nil)
	case errors.Is(err, domain.ErrDomainConflict):
		writeError(w, r, http.StatusConflict, "sender.domain_conflict", err.Error(), nil)
	case errors.Is(err, domain.ErrInvalidStateTransition):
		writeError(w, r, http.StatusConflict, "sender.invalid_state_transition", err.Error(), nil)
	case errors.Is(err, domain.ErrDomainInvalid):
		writeError(w, r, http.StatusUnprocessableEntity, "sender.domain_invalid", err.Error(), nil)
	case errors.Is(err, domain.ErrProviderConfigInvalid):
		writeError(w, r, http.StatusUnprocessableEntity, "sender.provider_config_invalid", err.Error(), nil)
	default:
		writeError(w, r, http.StatusInternalServerError, "health.runtime_not_ready", "internal error", nil)
	}
}

type desiredRecordDoc struct {
	RecordType string `json:"record_type"`
	Host       string `json:"host"`
	Value      string `json:"value"`
}

type currentStatusDoc struct {
	RecordType    string     `json:"record_type"`
	Host          string     `json:"host"`
	ExpectedValue string     `json:"expected_value"`
	CurrentValue  string     `json:"current_value"`
	Status        string     `json:"status"`
	LastCheckedAt *time.Time `json:"last_checked_at"`
	FailureReason string     `json:"failure_reason"`
}

type readinessDoc struct {
	Ready     bool   `json:"ready"`
	Reason    string `json:"reason"`
	CheckedAt string `json:"checked_at"`
}

func senderDomainResponse(result senderapp.Result) map[string]any {
	out := map[string]any{
		"id":           result.Domain.ID,
		"workspace_id": result.Domain.WorkspaceID,
		"domain":       result.Domain.Domain,
		"provider":     string(result.Domain.Provider),
		"status":       string(result.Domain.Status),
		"verified_at":  result.Domain.VerifiedAt,
		"disabled_at":  result.Domain.DisabledAt,
		"created_at":   result.Domain.CreatedAt,
		"updated_at":   result.Domain.UpdatedAt,
		"readiness": map[string]any{
			"ready":      result.Readiness.Ready,
			"reason":     result.Readiness.Reason,
			"checked_at": result.Readiness.CheckedAt.Format(time.RFC3339),
		},
	}

	desiredRecords := make([]desiredRecordDoc, 0, len(result.Records))
	currentStatus := make([]currentStatusDoc, 0, len(result.Records))
	for _, rec := range result.Records {
		desiredRecords = append(desiredRecords, desiredRecordDoc{
			RecordType: string(rec.RecordType),
			Host:       rec.Host,
			Value:      rec.ExpectedValue,
		})
		currentStatus = append(currentStatus, currentStatusDoc{
			RecordType:    string(rec.RecordType),
			Host:          rec.Host,
			ExpectedValue: rec.ExpectedValue,
			CurrentValue:  rec.CurrentValue,
			Status:        string(rec.Status),
			LastCheckedAt: rec.LastCheckedAt,
			FailureReason: rec.FailureReason,
		})
	}
	out["desired_records"] = desiredRecords
	out["current_status"] = currentStatus

	return out
}
