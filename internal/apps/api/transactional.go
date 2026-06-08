package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	deliveryapp "github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/app"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/domain"
)

type transactionalHTTP struct {
	svc *deliveryapp.Service
}

func newTransactionalHTTP(svc *deliveryapp.Service) *transactionalHTTP {
	return &transactionalHTTP{svc: svc}
}

type recipientInput struct {
	Email string `json:"email"`
	Name  string `json:"name,omitempty"`
}

type sendTransactionalInput struct {
	To                []recipientInput `json:"to"`
	SenderDomainID    string           `json:"sender_domain_id"`
	TemplateID        string           `json:"template_id"`
	TemplateVersionID string           `json:"template_version_id,omitempty"`
	TemplateData      map[string]any   `json:"template_data,omitempty"`
	Metadata          map[string]any   `json:"metadata,omitempty"`
	Tags              []string         `json:"tags,omitempty"`
	MessageType       string           `json:"message_type,omitempty"`
}

func (h *transactionalHTTP) send(w http.ResponseWriter, r *http.Request) {
	workspaceID, _ := r.Context().Value(ctxAPIKeyWorkspaceID).(string)
	apiKeyID, _ := r.Context().Value(ctxAPIKeyID).(string)

	if workspaceID == "" {
		writeError(w, r, http.StatusUnauthorized, "api_key.invalid", "missing workspace context", nil)
		return
	}

	idempotencyKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))

	var req sendTransactionalInput
	if !decodeJSON(w, r, &req) {
		return
	}

	if req.MessageType != "" && req.MessageType != "transactional" {
		writeError(w, r, http.StatusBadRequest, "delivery.request_body_invalid", "message_type must be 'transactional' if provided", nil)
		return
	}

	if len(req.To) != 1 {
		writeError(w, r, http.StatusUnprocessableEntity, "delivery.recipient_invalid", "exactly one recipient is required", nil)
		return
	}

	recipientEmail := strings.TrimSpace(req.To[0].Email)
	if recipientEmail == "" {
		writeError(w, r, http.StatusUnprocessableEntity, "delivery.recipient_invalid", "recipient email is required", nil)
		return
	}

	result, err := h.svc.AcceptTransactionalSend(r.Context(), deliveryapp.AcceptTransactionalSendInput{
		WorkspaceID:       workspaceID,
		APIKeyID:          apiKeyID,
		IdempotencyKey:    idempotencyKey,
		RecipientEmail:    recipientEmail,
		RecipientName:     req.To[0].Name,
		SenderDomainID:    req.SenderDomainID,
		TemplateID:        req.TemplateID,
		TemplateVersionID: req.TemplateVersionID,
		TemplateData:      req.TemplateData,
		Metadata:          req.Metadata,
		Tags:              req.Tags,
		Now:               time.Now().UTC(),
	})
	if err != nil {
		writeTransactionalErr(w, r, err)
		return
	}

	writeEnvelope(w, r, http.StatusAccepted, map[string]any{
		"message_id":  result.MessageID,
		"request_id":  result.RequestID,
		"status":      result.Status,
		"accepted_at": result.AcceptedAt,
	})
}

func (h *transactionalHTTP) getMessage(w http.ResponseWriter, r *http.Request) {
	workspaceID, _ := r.Context().Value(ctxAPIKeyWorkspaceID).(string)
	messageID := strings.TrimPrefix(r.URL.Path, "/api/v1/transactional/messages/")
	messageID = strings.TrimSuffix(messageID, "/")

	if workspaceID == "" {
		writeError(w, r, http.StatusUnauthorized, "api_key.invalid", "missing workspace context", nil)
		return
	}

	if messageID == "" {
		writeError(w, r, http.StatusNotFound, "delivery.message_not_found", "message_id is required", nil)
		return
	}

	result, err := h.svc.GetTransactionalMessage(r.Context(), deliveryapp.GetTransactionalMessageInput{
		WorkspaceID: workspaceID,
		MessageID:   messageID,
	})
	if err != nil {
		writeTransactionalErr(w, r, err)
		return
	}

	writeEnvelope(w, r, http.StatusOK, map[string]any{
		"message_id":          result.MessageID,
		"status":              result.Status,
		"provider":            result.Provider,
		"provider_message_id": result.ProviderMessageID,
		"last_updated_at":     result.LastUpdatedAt,
	})
}

func writeTransactionalErr(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrRequestBodyInvalid):
		writeError(w, r, http.StatusBadRequest, "delivery.request_body_invalid", err.Error(), nil)
	case errors.Is(err, domain.ErrMessageNotFound):
		writeError(w, r, http.StatusNotFound, "delivery.message_not_found", err.Error(), nil)
	case errors.Is(err, domain.ErrIdempotencyKeyConflict):
		writeError(w, r, http.StatusConflict, "delivery.idempotency_key_conflict", err.Error(), nil)
	case errors.Is(err, domain.ErrRecipientInvalid):
		writeError(w, r, http.StatusUnprocessableEntity, "delivery.recipient_invalid", err.Error(), nil)
	case errors.Is(err, domain.ErrSuppressedRecipient):
		writeError(w, r, http.StatusUnprocessableEntity, "delivery.recipient_suppressed", err.Error(), nil)
	case errors.Is(err, domain.ErrSenderDomainNotFound):
		writeError(w, r, http.StatusNotFound, "sender.domain_not_found", err.Error(), nil)
	case errors.Is(err, domain.ErrSenderDomainNotVerified):
		writeError(w, r, http.StatusUnprocessableEntity, "sender.domain_not_verified", err.Error(), nil)
	case errors.Is(err, domain.ErrTemplateNotFound):
		writeError(w, r, http.StatusNotFound, "template.not_found", err.Error(), nil)
	case errors.Is(err, domain.ErrTemplateRenderPayloadInvalid):
		writeError(w, r, http.StatusUnprocessableEntity, "template.render_payload_invalid", err.Error(), nil)
	case errors.Is(err, domain.ErrTemporarilyUnavailable):
		writeError(w, r, http.StatusServiceUnavailable, "delivery.temporarily_unavailable", err.Error(), nil)
	default:
		writeError(w, r, http.StatusInternalServerError, "health.runtime_not_ready", "internal error", nil)
	}
}
