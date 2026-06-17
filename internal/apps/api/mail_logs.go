package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	deliveryapp "github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/app"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/constants"
)

type mailLogsHTTP struct {
	svc deliveryService
}

func newMailLogsHTTP(svc deliveryService) *mailLogsHTTP {
	return &mailLogsHTTP{svc: svc}
}

// resolveWorkspaceID extracts the workspace ID from either the Chi URL param
// (session-auth flows) or the context value (API-key flows).
func resolveWorkspaceID(r *http.Request) string {
	if ws := chi.URLParam(r, "workspace_id"); ws != "" {
		return ws
	}
	if ws, ok := r.Context().Value(ctxAPIKeyWorkspaceID).(string); ok && ws != "" {
		return ws
	}
	return ""
}

// listMailLogs returns a paginated, filterable list of workspace messages.
// Supports both session-auth and API-key (mail_logs.read) auth.
func (h *mailLogsHTTP) listMailLogs(w http.ResponseWriter, r *http.Request) {
	workspaceID := resolveWorkspaceID(r)
	if workspaceID == "" {
		writeError(w, r, http.StatusUnauthorized, "api_key.invalid", "missing workspace context", nil)
		return
	}

	q := r.URL.Query()

	var from, to *time.Time
	if s := q.Get("from"); s != "" {
		t, err := time.Parse(time.RFC3339, s)
		if err == nil {
			from = &t
		}
	}
	if s := q.Get("to"); s != "" {
		t, err := time.Parse(time.RFC3339, s)
		if err == nil {
			to = &t
		}
	}

	limit := parseLimitParam(q.Get("limit"), constants.DefaultPageSize, 100)

	results, err := h.svc.ListMessages(r.Context(), deliveryapp.ListMessagesInput{
		WorkspaceID:              workspaceID,
		CampaignID:               q.Get("campaign_id"),
		TransactionalRequestID:   q.Get("transactional_request_id"),
		Status:                   q.Get("status"),
		MessageType:              q.Get("message_type"),
		Mode:                     q.Get("mode"),
		Provider:                 q.Get("provider"),
		RecipientEmailNormalized: q.Get("recipient_email"),
		ProviderMessageID:        q.Get("provider_message_id"),
		From:                     from,
		To:                       to,
		Limit:                    limit,
		Cursor:                   q.Get("cursor"),
	})
	if err != nil {
		writeMailLogErr(w, r, err)
		return
	}

	out := make([]map[string]any, 0, len(results.Messages))
	for _, msg := range results.Messages {
		out = append(out, mailLogSummaryResponse(msg))
	}
	writeEnvelope(w, r, http.StatusOK, map[string]any{
		"messages":    out,
		"next_cursor": results.NextCursor,
	})
}

// getMailLog returns a single message detail with full context.
func (h *mailLogsHTTP) getMailLog(w http.ResponseWriter, r *http.Request) {
	workspaceID := resolveWorkspaceID(r)
	messageID := chi.URLParam(r, "message_id")

	if workspaceID == "" {
		writeError(w, r, http.StatusUnauthorized, "api_key.invalid", "missing workspace context", nil)
		return
	}
	if messageID == "" {
		writeError(w, r, http.StatusNotFound, "delivery.message_not_found", "message_id is required", nil)
		return
	}

	msg, err := h.svc.GetMessage(r.Context(), deliveryapp.GetMessageInput{
		WorkspaceID: workspaceID,
		MessageID:   messageID,
	})
	if err != nil {
		writeMailLogErr(w, r, err)
		return
	}

	writeEnvelope(w, r, http.StatusOK, mailLogDetailResponse(*msg))
}

// listMailLogAttempts returns all delivery attempts for a message.
func (h *mailLogsHTTP) listMailLogAttempts(w http.ResponseWriter, r *http.Request) {
	workspaceID := resolveWorkspaceID(r)
	messageID := chi.URLParam(r, "message_id")

	if workspaceID == "" {
		writeError(w, r, http.StatusUnauthorized, "api_key.invalid", "missing workspace context", nil)
		return
	}
	if messageID == "" {
		writeError(w, r, http.StatusNotFound, "delivery.message_not_found", "message_id is required", nil)
		return
	}

	attempts, err := h.svc.ListAttempts(r.Context(), deliveryapp.ListAttemptsInput{
		WorkspaceID: workspaceID,
		MessageID:   messageID,
	})
	if err != nil {
		writeMailLogErr(w, r, err)
		return
	}

	out := make([]map[string]any, 0, len(attempts))
	for _, a := range attempts {
		out = append(out, map[string]any{
			"id":                a.ID,
			"attempt_no":        a.AttemptNo,
			"provider":          a.Provider,
			"status":            a.Status,
			"error_class":       a.ErrorClass,
			"error_message":     a.ErrorMessage,
			"started_at":        a.StartedAt,
			"finished_at":       a.FinishedAt,
			"request_snapshot":  a.RequestSnapshot,
			"response_snapshot": a.ResponseSnapshot,
		})
	}
	writeEnvelope(w, r, http.StatusOK, map[string]any{
		"attempts": out,
	})
}

// listMailLogEvents returns the timeline events for a message.
func (h *mailLogsHTTP) listMailLogEvents(w http.ResponseWriter, r *http.Request) {
	workspaceID := resolveWorkspaceID(r)
	messageID := chi.URLParam(r, "message_id")

	if workspaceID == "" {
		writeError(w, r, http.StatusUnauthorized, "api_key.invalid", "missing workspace context", nil)
		return
	}
	if messageID == "" {
		writeError(w, r, http.StatusNotFound, "delivery.message_not_found", "message_id is required", nil)
		return
	}

	result, err := h.svc.ListMessageEvents(r.Context(), deliveryapp.ListMessageEventsInput{
		WorkspaceID: workspaceID,
		MessageID:   messageID,
	})
	if err != nil {
		writeMailLogErr(w, r, err)
		return
	}

	writeEnvelope(w, r, http.StatusOK, map[string]any{
		"events":      result.Events,
		"next_cursor": result.NextCursor,
	})
}

func writeMailLogErr(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrReadDenied):
		writeError(w, r, http.StatusForbidden, "delivery.read_denied", err.Error(), nil)
	case errors.Is(err, domain.ErrPayloadInvalid):
		writeError(w, r, http.StatusUnprocessableEntity, "delivery.query_invalid", err.Error(), nil)
	case errors.Is(err, domain.ErrMessageNotFound):
		writeError(w, r, http.StatusNotFound, "delivery.message_not_found", err.Error(), nil)
	default:
		writeError(w, r, http.StatusInternalServerError, "internal.error", "internal error", nil)
	}
}

func mailLogSummaryResponse(m domain.Message) map[string]any {
	resp := map[string]any{
		"id":                         m.ID,
		"workspace_id":               m.WorkspaceID,
		"transactional_request_id":   m.TransactionalRequestID,
		"recipient_email":            m.RecipientSnapshot.Email,
		"recipient_email_normalized": m.RecipientEmailNormalized,
		"recipient_role":             m.RecipientRole,
		"subject":                    m.Subject,
		"sender_name":                m.SenderName,
		"message_type":               m.MessageType,
		"source_type":                m.SourceType,
		"status":                     m.Status,
		"scheduled_at":               m.ScheduledAt,
		"queued_at":                  m.QueuedAt,
		"processing_started_at":      m.ProcessingStartedAt,
		"accepted_at":                m.AcceptedAt,
		"delivered_at":               m.DeliveredAt,
		"bounced_at":                 m.BouncedAt,
		"complained_at":              m.ComplainedAt,
		"failed_at":                  m.FailedAt,
		"last_error_class":           m.LastErrorClass,
		"last_error_message":         m.LastErrorMessage,
		"provider":                   m.Provider,
		"provider_message_id":        m.ProviderMessageID,
		"created_at":                 m.CreatedAt,
		"updated_at":                 m.UpdatedAt,
	}
	// Derive mode from template_id presence for display purposes
	if m.TemplateID != "" {
		resp["mode"] = "template"
	} else {
		resp["mode"] = "raw"
	}
	return resp
}

func mailLogDetailResponse(m domain.Message) map[string]any {
	resp := mailLogSummaryResponse(m)
	resp["recipient_snapshot"] = m.RecipientSnapshot
	resp["template_id"] = m.TemplateID
	resp["template_version_id"] = m.TemplateVersionID
	resp["sender_domain_id"] = m.SenderDomainID
	resp["campaign_id"] = m.CampaignID
	resp["contact_id"] = m.ContactID
	return resp
}
