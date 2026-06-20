package api

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	auditapp "github.com/ninggiangboy/send-flow/backend/internal/modules/audit/app"
	auditdomain "github.com/ninggiangboy/send-flow/backend/internal/modules/audit/domain"
)

type auditHTTP struct {
	svc *auditapp.Service
}

func newAuditHTTP(svc *auditapp.Service) *auditHTTP {
	return &auditHTTP{svc: svc}
}

type auditEntryResponse struct {
	ID             string         `json:"id"`
	ActorUserID    string         `json:"actor_user_id,omitempty"`
	ActionType     string         `json:"action_type"`
	TargetType     string         `json:"target_type,omitempty"`
	TargetID       string         `json:"target_id,omitempty"`
	PayloadSummary map[string]any `json:"payload_summary,omitempty"`
	RequestID      string         `json:"request_id,omitempty"`
	OccurredAt     time.Time      `json:"occurred_at"`
}

type auditListResponse struct {
	Entries    []auditEntryResponse `json:"entries"`
	NextCursor string               `json:"next_cursor"`
}

func (h *auditHTTP) listAuditLogs(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	userID, _ := r.Context().Value(ctxUserID).(string)

	q := r.URL.Query()
	limit := 50
	if l := q.Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	var from, to *time.Time
	if f := q.Get("from"); f != "" {
		if parsed, err := time.Parse(time.RFC3339, f); err == nil {
			from = &parsed
		}
	}
	if t := q.Get("to"); t != "" {
		if parsed, err := time.Parse(time.RFC3339, t); err == nil {
			to = &parsed
		}
	}

	entries, next, err := h.svc.ListAuditEntries(r.Context(), auditapp.ListAuditEntriesInput{
		WorkspaceID: workspaceID,
		UserID:      userID,
		ActorUserID: q.Get("actor_user_id"),
		ActionType:  q.Get("action_type"),
		TargetType:  q.Get("target_type"),
		TargetID:    q.Get("target_id"),
		From:        from,
		To:          to,
		Limit:       limit,
		Cursor:      q.Get("cursor"),
	})
	if err != nil {
		writeAuditErr(w, r, err)
		return
	}

	resp := auditListResponse{
		Entries:    make([]auditEntryResponse, len(entries)),
		NextCursor: next,
	}
	for i, e := range entries {
		resp.Entries[i] = auditEntryToResponse(e)
	}

	writeEnvelope(w, r, http.StatusOK, resp)
}

func auditEntryToResponse(e auditdomain.AuditEntry) auditEntryResponse {
	return auditEntryResponse{
		ID:             e.ID,
		ActorUserID:    e.ActorUserID,
		ActionType:     e.ActionType,
		TargetType:     e.TargetType,
		TargetID:       e.TargetID,
		PayloadSummary: e.PayloadSummary,
		RequestID:      e.RequestID,
		OccurredAt:     e.OccurredAt,
	}
}

func writeAuditErr(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, auditdomain.ErrAuditReadDenied):
		writeError(w, r, http.StatusForbidden, "audit.read_denied", err.Error(), nil)
	case errors.Is(err, auditdomain.ErrAuditFilterInvalid):
		writeError(w, r, http.StatusBadRequest, "audit.filter_invalid", err.Error(), nil)
	default:
		writeInternalError(w, r)
	}
}
