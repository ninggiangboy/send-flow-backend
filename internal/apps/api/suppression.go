package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/suppression/app"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/suppression/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/suppression/ports"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/auth"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/constants"
)

type suppressionHTTP struct {
	svc *app.Service
}

func newSuppressionHTTP(svc *app.Service) *suppressionHTTP {
	return &suppressionHTTP{svc: svc}
}

func (h *suppressionHTTP) listSuppressionEntries(w http.ResponseWriter, r *http.Request) {
	workspaceID := workspaceIDParam(r)
	userID, _ := r.Context().Value(ctxUserID).(string)

	q := r.URL.Query()
	limit := parseLimitParam(q.Get("limit"), constants.DefaultPageSize, 100)
	if limit > 100 {
		limit = 100
	}

	var from, to *time.Time
	if f := q.Get("from"); f != "" {
		t, err := time.Parse(time.RFC3339, f)
		if err == nil {
			from = &t
		}
	}
	if t := q.Get("to"); t != "" {
		parsed, err := time.Parse(time.RFC3339, t)
		if err == nil {
			to = &parsed
		}
	}

	query := ports.SuppressionListQuery{
		WorkspaceID: workspaceID,
		Email:       q.Get("email"),
		Scope:       q.Get("scope"),
		Reason:      q.Get("reason"),
		From:        from,
		To:          to,
		Cursor:      q.Get("cursor"),
		Limit:       limit,
	}

	result, err := h.svc.ListEntries(r.Context(), query, userID)
	if err != nil {
		writeSuppressionErr(w, r, err)
		return
	}

	out := make([]map[string]any, 0, len(result.Entries))
	for _, e := range result.Entries {
		out = append(out, suppressionEntryResponse(e))
	}

	if result.NextCursor != "" {
		w.Header().Set("X-Next-Cursor", result.NextCursor)
	}
	writeEnvelope(w, r, http.StatusOK, out)
}

func (h *suppressionHTTP) createSuppressionEntry(w http.ResponseWriter, r *http.Request) {
	workspaceID := workspaceIDParam(r)
	userID, _ := r.Context().Value(ctxUserID).(string)

	var req struct {
		Email  string `json:"email"`
		Scope  string `json:"scope"`
		Reason string `json:"reason"`
		Note   string `json:"note"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	entry, err := h.svc.CreateEntry(r.Context(), app.CreateEntryInput{
		WorkspaceID: workspaceID,
		UserID:      userID,
		Email:       req.Email,
		Scope:       req.Scope,
		Reason:      req.Reason,
		Note:        req.Note,
		Now:         time.Now().UTC(),
	})
	if err != nil {
		writeSuppressionErr(w, r, err)
		return
	}

	writeEnvelope(w, r, http.StatusCreated, suppressionEntryResponse(*entry))
}

func (h *suppressionHTTP) deleteSuppressionEntry(w http.ResponseWriter, r *http.Request) {
	workspaceID := workspaceIDParam(r)
	entryID := pathParam(r, "suppression_id")
	userID, _ := r.Context().Value(ctxUserID).(string)

	if err := h.svc.RemoveEntry(r.Context(), workspaceID, entryID, userID, time.Now().UTC()); err != nil {
		writeSuppressionErr(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// --- Error mapping ---

func writeSuppressionErr(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrReadDenied):
		writeError(w, r, http.StatusForbidden, errCodeSuppressionReadDenied, err.Error(), nil)
	case errors.Is(err, domain.ErrManageDenied):
		writeError(w, r, http.StatusForbidden, errCodeSuppressionManageDenied, err.Error(), nil)
	case errors.Is(err, domain.ErrEntryNotFound):
		writeError(w, r, http.StatusNotFound, errCodeSuppressionEntryNotFound, err.Error(), nil)
	case errors.Is(err, domain.ErrUnsuppressConflict):
		writeError(w, r, http.StatusConflict, errCodeSuppressionUnsuppressConflict, err.Error(), nil)
	case errors.Is(err, domain.ErrScopeInvalid):
		writeError(w, r, http.StatusUnprocessableEntity, errCodeSuppressionScopeInvalid, err.Error(), nil)
	case errors.Is(err, domain.ErrReasonInvalid):
		writeError(w, r, http.StatusUnprocessableEntity, errCodeSuppressionReasonInvalid, err.Error(), nil)
	case errors.Is(err, auth.ErrPermissionDenied):
		writeError(w, r, http.StatusForbidden, errCodeAuthPermissionDenied, err.Error(), nil)
	default:
		writeInternalError(w, r)
	}
}

// --- Response mappers ---

func suppressionEntryResponse(e domain.SuppressionEntry) map[string]any {
	return map[string]any{
		"id":           e.ID,
		"workspace_id": e.WorkspaceID,
		"email":        e.Email,
		"scope":        string(e.Scope),
		"reason":       string(e.Reason),
		"status":       string(e.Status),
		"note":         e.Note,
		"created_at":   e.CreatedAt,
		"updated_at":   e.UpdatedAt,
		"removed_at":   e.RemovedAt,
	}
}
