package api

import (
	"errors"
	"net/http"
	"time"

	deliveryapp "github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/app"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/auth"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/constants"
)

type deliveryHTTP struct {
	svc deliveryService
}

func newDeliveryHTTP(svc deliveryService) *deliveryHTTP {
	return &deliveryHTTP{svc: svc}
}

func (h *deliveryHTTP) listMessages(w http.ResponseWriter, r *http.Request) {
	workspaceID := workspaceIDParam(r)
	userID, _ := r.Context().Value(ctxUserID).(string)

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
		UserID:                   userID,
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
		writeDeliveryErr(w, r, err)
		return
	}

	out := make([]map[string]any, 0, len(results.Messages))
	for _, msg := range results.Messages {
		out = append(out, messageSummaryResponse(msg))
	}
	writeEnvelope(w, r, http.StatusOK, map[string]any{"messages": out, "next_cursor": results.NextCursor})
}

func (h *deliveryHTTP) getMessage(w http.ResponseWriter, r *http.Request) {
	workspaceID := workspaceIDParam(r)
	messageID := messageIDParam(r)
	userID, _ := r.Context().Value(ctxUserID).(string)

	msg, err := h.svc.GetMessage(r.Context(), deliveryapp.GetMessageInput{
		UserID:      userID,
		WorkspaceID: workspaceID,
		MessageID:   messageID,
	})
	if err != nil {
		writeDeliveryErr(w, r, err)
		return
	}

	writeEnvelope(w, r, http.StatusOK, messageDetailResponse(*msg))
}

func writeDeliveryErr(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrReadDenied):
		writeError(w, r, http.StatusForbidden, errCodeDeliveryReadDenied, err.Error(), nil)
	case errors.Is(err, domain.ErrPayloadInvalid):
		writeError(w, r, http.StatusUnprocessableEntity, errCodeDeliveryQueryInvalid, err.Error(), nil)
	case errors.Is(err, domain.ErrMessageNotFound):
		writeError(w, r, http.StatusNotFound, errCodeDeliveryMessageNotFound, err.Error(), nil)
	case errors.Is(err, auth.ErrPermissionDenied):
		writeError(w, r, http.StatusForbidden, errCodeAuthPermissionDenied, err.Error(), nil)
	default:
		writeInternalError(w, r)
	}
}

func messageSummaryResponse(m domain.Message) map[string]any {
	return map[string]any{
		"id":                         m.ID,
		"workspace_id":               m.WorkspaceID,
		"campaign_id":                m.CampaignID,
		"campaign_candidate_id":      m.CampaignCandidateID,
		"transactional_request_id":   m.TransactionalRequestID,
		"contact_id":                 m.ContactID,
		"recipient_email_normalized": m.RecipientEmailNormalized,
		"template_id":                m.TemplateID,
		"template_version_id":        m.TemplateVersionID,
		"sender_domain_id":           m.SenderDomainID,
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
}

func messageDetailResponse(m domain.Message) map[string]any {
	resp := messageSummaryResponse(m)
	resp["recipient_snapshot"] = m.RecipientSnapshot
	return resp
}
