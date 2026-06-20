package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	deliveryapp "github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/app"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/app/send"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/domain"
	identityapp "github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app"
)

const (
	maxMultipartMemory = 32 << 20 // 32 MB
	maxAttachmentSize  = 25 << 20 // 25 MB
)

type transactionalHTTP struct {
	svc           deliveryService
	auditRecorder identityapp.AuditRecorder
}

func newTransactionalHTTP(svc deliveryService, auditRecorder identityapp.AuditRecorder) *transactionalHTTP {
	return &transactionalHTTP{svc: svc, auditRecorder: auditRecorder}
}

type recipientInput struct {
	Email string `json:"email"`
	Name  string `json:"name,omitempty"`
}

type sendTransactionalInput struct {
	Mode              string            `json:"mode"`
	To                []recipientInput  `json:"to"`
	CC                []recipientInput  `json:"cc,omitempty"`
	BCC               []recipientInput  `json:"bcc,omitempty"`
	SenderDomainID    string            `json:"sender_domain_id"`
	SenderName        string            `json:"sender_name,omitempty"`
	Subject           string            `json:"subject,omitempty"`
	TemplateID        string            `json:"template_id,omitempty"`
	TemplateVersionID string            `json:"template_version_id,omitempty"`
	TemplateData      map[string]any    `json:"template_data,omitempty"`
	TextBody          string            `json:"text_body,omitempty"`
	HTMLBody          string            `json:"html_body,omitempty"`
	ReplyTo           string            `json:"reply_to,omitempty"`
	Headers           map[string]string `json:"headers,omitempty"`
	Metadata          map[string]any    `json:"metadata,omitempty"`
	Tags              []string          `json:"tags,omitempty"`
}

func (h *transactionalHTTP) recordAudit(r *http.Request, input identityapp.RecordAuditInput) {
	if h.auditRecorder == nil {
		return
	}
	reqCtx, ok := r.Context().Value(ctxRequestContext).(*requestLogContext)
	if ok {
		input.RequestID = reqCtx.RequestID
	}
	input.OccurredAt = time.Now().UTC()
	if err := h.auditRecorder.Record(r.Context(), input); err != nil {
		slog.Warn("failed to record audit event", "error", err)
	}
}

func (h *transactionalHTTP) send(w http.ResponseWriter, r *http.Request) {
	workspaceID, _ := r.Context().Value(ctxAPIKeyWorkspaceID).(string)
	apiKeyID, _ := r.Context().Value(ctxAPIKeyID).(string)

	if workspaceID == "" {
		writeError(w, r, http.StatusUnauthorized, errCodeAPIKeyInvalid, "missing workspace context", nil)
		return
	}

	idempotencyKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	contentType := headerContentType(r)

	var input send.Input
	var err error

	if strings.HasPrefix(contentType, "multipart/form-data") {
		input, err = h.parseMultipartSend(r)
	} else if strings.HasPrefix(contentType, "application/json") {
		input, err = h.parseJSONSend(r)
	} else {
		writeError(w, r, http.StatusUnsupportedMediaType, errCodeDeliveryRequestBodyInvalid, "content type must be application/json or multipart/form-data", nil)
		return
	}

	if err != nil {
		writeTransactionalErr(w, r, err)
		return
	}

	input.WorkspaceID = workspaceID
	input.APIKeyID = apiKeyID
	input.IdempotencyKey = idempotencyKey
	input.Now = time.Now().UTC()

	result, err := h.svc.AcceptTransactionalSend(r.Context(), input)
	if err != nil {
		writeTransactionalErr(w, r, err)
		return
	}

	h.recordAudit(r, identityapp.RecordAuditInput{
		WorkspaceID: workspaceID,
		ActorUserID: apiKeyID,
		ActionType:  auditActionTransactionalSendAccepted,
		TargetType:  "transactional_send_request",
		TargetID:    result.RequestID,
		PayloadSummary: map[string]any{
			"mode":          input.Mode,
			"message_count": len(result.MessageIDs),
			"api_key_id":    apiKeyID,
		},
	})

	resp := map[string]any{
		"request_id":  result.RequestID,
		"message_ids": result.MessageIDs,
		"status":      result.Status,
		"accepted_at": result.AcceptedAt,
	}
	if len(result.MessageIDs) == 1 {
		resp["message_id"] = result.MessageIDs[0]
	}

	writeEnvelope(w, r, http.StatusAccepted, resp)
}

func (h *transactionalHTTP) parseJSONSend(r *http.Request) (send.Input, error) {
	var req sendTransactionalInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return send.Input{}, domain.ErrRequestBodyInvalid
	}

	input := send.Input{
		Mode:              req.Mode,
		SenderDomainID:    req.SenderDomainID,
		SenderName:        req.SenderName,
		Subject:           req.Subject,
		TemplateID:        req.TemplateID,
		TemplateVersionID: req.TemplateVersionID,
		TemplateData:      req.TemplateData,
		TextBody:          req.TextBody,
		HTMLBody:          req.HTMLBody,
		ReplyTo:           req.ReplyTo,
		Metadata:          req.Metadata,
		Tags:              req.Tags,
		Headers:           req.Headers,
	}

	for _, r := range req.To {
		input.To = append(input.To, domain.RecipientTarget{Email: r.Email, Name: r.Name})
	}
	for _, r := range req.CC {
		input.CC = append(input.CC, domain.RecipientTarget{Email: r.Email, Name: r.Name})
	}
	for _, r := range req.BCC {
		input.BCC = append(input.BCC, domain.RecipientTarget{Email: r.Email, Name: r.Name})
	}

	// Default mode to template for backward compatibility
	if input.Mode == "" {
		input.Mode = domain.MessageModeTemplate
	}

	return input, nil
}

func (h *transactionalHTTP) parseMultipartSend(r *http.Request) (send.Input, error) {
	if err := r.ParseMultipartForm(maxMultipartMemory); err != nil {
		return send.Input{}, domain.ErrRequestBodyInvalid
	}

	var input send.Input
	input.Mode = r.FormValue("mode")
	if input.Mode == "" {
		input.Mode = domain.MessageModeTemplate
	}

	input.SenderDomainID = r.FormValue("sender_domain_id")
	input.SenderName = r.FormValue("sender_name")
	input.Subject = r.FormValue("subject")
	input.TemplateID = r.FormValue("template_id")
	input.TemplateVersionID = r.FormValue("template_version_id")
	input.TextBody = r.FormValue("text_body")
	input.HTMLBody = r.FormValue("html_body")
	input.ReplyTo = r.FormValue("reply_to")

	// Parse JSON fields
	if v := r.FormValue("to"); v != "" {
		var to []recipientInput
		if err := json.Unmarshal([]byte(v), &to); err != nil {
			return send.Input{}, domain.ErrRequestBodyInvalid
		}
		for _, r := range to {
			input.To = append(input.To, domain.RecipientTarget{Email: r.Email, Name: r.Name})
		}
	}
	if v := r.FormValue("cc"); v != "" {
		var cc []recipientInput
		if err := json.Unmarshal([]byte(v), &cc); err != nil {
			return send.Input{}, domain.ErrRequestBodyInvalid
		}
		for _, r := range cc {
			input.CC = append(input.CC, domain.RecipientTarget{Email: r.Email, Name: r.Name})
		}
	}
	if v := r.FormValue("bcc"); v != "" {
		var bcc []recipientInput
		if err := json.Unmarshal([]byte(v), &bcc); err != nil {
			return send.Input{}, domain.ErrRequestBodyInvalid
		}
		for _, r := range bcc {
			input.BCC = append(input.BCC, domain.RecipientTarget{Email: r.Email, Name: r.Name})
		}
	}
	if v := r.FormValue("headers"); v != "" {
		var headers map[string]string
		if err := json.Unmarshal([]byte(v), &headers); err != nil {
			return send.Input{}, domain.ErrRequestBodyInvalid
		}
		input.Headers = headers
	}
	if v := r.FormValue("metadata"); v != "" {
		var metadata map[string]any
		if err := json.Unmarshal([]byte(v), &metadata); err != nil {
			return send.Input{}, domain.ErrRequestBodyInvalid
		}
		input.Metadata = metadata
	}
	if v := r.FormValue("tags"); v != "" {
		var tags []string
		if err := json.Unmarshal([]byte(v), &tags); err != nil {
			return send.Input{}, domain.ErrRequestBodyInvalid
		}
		input.Tags = tags
	}
	if v := r.FormValue("template_data"); v != "" {
		var data map[string]any
		if err := json.Unmarshal([]byte(v), &data); err != nil {
			return send.Input{}, domain.ErrRequestBodyInvalid
		}
		input.TemplateData = data
	}

	// Parse file attachments
	fileHeaders := r.MultipartForm.File["attachments"]
	for _, fh := range fileHeaders {
		att, err := parseAttachmentPart(fh)
		if err != nil {
			return send.Input{}, err
		}
		input.Attachments = append(input.Attachments, att)
	}

	return input, nil
}

func parseAttachmentPart(fh *multipart.FileHeader) (send.AttachmentStream, error) {
	if fh.Size > maxAttachmentSize {
		return send.AttachmentStream{}, domain.ErrAttachmentTooLarge
	}

	file, err := fh.Open()
	if err != nil {
		return send.AttachmentStream{}, domain.ErrRequestBodyInvalid
	}
	defer file.Close()

	// Compute SHA256 while reading file content
	hash := sha256.New()
	teeReader := io.TeeReader(file, hash)
	content, err := io.ReadAll(io.LimitReader(teeReader, maxAttachmentSize+1))
	if err != nil {
		return send.AttachmentStream{}, domain.ErrRequestBodyInvalid
	}

	return send.AttachmentStream{
		Filename:    fh.Filename,
		ContentType: fh.Header.Get("Content-Type"),
		Data:        bytes.NewReader(content),
		Size:        int64(len(content)),
		SHA256:      hex.EncodeToString(hash.Sum(nil)),
		Disposition: "attachment",
	}, nil
}

func (h *transactionalHTTP) getMessage(w http.ResponseWriter, r *http.Request) {
	workspaceID, _ := r.Context().Value(ctxAPIKeyWorkspaceID).(string)
	messageID := strings.TrimPrefix(r.URL.Path, "/api/v1/transactional/messages/")
	messageID = strings.TrimSuffix(messageID, "/")

	if workspaceID == "" {
		writeError(w, r, http.StatusUnauthorized, errCodeAPIKeyInvalid, "missing workspace context", nil)
		return
	}

	if messageID == "" {
		writeError(w, r, http.StatusNotFound, errCodeDeliveryMessageNotFound, "message_id is required", nil)
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

// listMessageEvents returns the timeline of events for a specific message.
// Requires API key with mail_logs.read scope.
func (h *transactionalHTTP) listMessageEvents(w http.ResponseWriter, r *http.Request) {
	workspaceID, _ := r.Context().Value(ctxAPIKeyWorkspaceID).(string)
	messageID := messageIDParam(r)

	if workspaceID == "" {
		writeError(w, r, http.StatusUnauthorized, errCodeAPIKeyInvalid, "missing workspace context", nil)
		return
	}
	if messageID == "" {
		writeError(w, r, http.StatusNotFound, errCodeDeliveryMessageNotFound, "message_id is required", nil)
		return
	}

	result, err := h.svc.ListMessageEvents(r.Context(), deliveryapp.ListMessageEventsInput{
		WorkspaceID: workspaceID,
		MessageID:   messageID,
	})
	if err != nil {
		writeTransactionalErr(w, r, err)
		return
	}

	writeEnvelope(w, r, http.StatusOK, map[string]any{
		"events": result.Events,
	})
}

// listRequestMessages returns all messages for a transactional request.
// Requires API key with mail_logs.read scope.
func (h *transactionalHTTP) listRequestMessages(w http.ResponseWriter, r *http.Request) {
	workspaceID, _ := r.Context().Value(ctxAPIKeyWorkspaceID).(string)
	requestID := requestIDParam(r)

	if workspaceID == "" {
		writeError(w, r, http.StatusUnauthorized, errCodeAPIKeyInvalid, "missing workspace context", nil)
		return
	}
	if requestID == "" {
		writeError(w, r, http.StatusNotFound, errCodeDeliveryTransactionalRequestNotFound, "request_id is required", nil)
		return
	}

	result, err := h.svc.ListRequestMessages(r.Context(), deliveryapp.ListRequestMessagesInput{
		WorkspaceID: workspaceID,
		RequestID:   requestID,
	})
	if err != nil {
		writeTransactionalErr(w, r, err)
		return
	}

	writeEnvelope(w, r, http.StatusOK, map[string]any{
		"messages": result.Messages,
	})
}

func writeTransactionalErr(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrRequestBodyInvalid):
		writeError(w, r, http.StatusBadRequest, errCodeDeliveryRequestBodyInvalid, err.Error(), nil)
	case errors.Is(err, domain.ErrMessageNotFound):
		writeError(w, r, http.StatusNotFound, errCodeDeliveryMessageNotFound, err.Error(), nil)
	case errors.Is(err, domain.ErrIdempotencyKeyConflict):
		writeError(w, r, http.StatusConflict, errCodeDeliveryIdempotencyKeyConflict, err.Error(), nil)
	case errors.Is(err, domain.ErrRecipientInvalid):
		writeError(w, r, http.StatusUnprocessableEntity, errCodeDeliveryRecipientInvalid, err.Error(), nil)
	case errors.Is(err, domain.ErrSuppressedRecipient):
		writeError(w, r, http.StatusUnprocessableEntity, errCodeDeliveryRecipientSuppressed, err.Error(), nil)
	case errors.Is(err, domain.ErrSenderDomainNotFound):
		writeError(w, r, http.StatusNotFound, errCodeSenderDomainNotFound, err.Error(), nil)
	case errors.Is(err, domain.ErrSenderDomainNotVerified):
		writeError(w, r, http.StatusUnprocessableEntity, errCodeSenderDomainNotVerified, err.Error(), nil)
	case errors.Is(err, domain.ErrTemplateNotFound):
		writeError(w, r, http.StatusNotFound, errCodeTemplateNotFound, err.Error(), nil)
	case errors.Is(err, domain.ErrTemplateRenderPayloadInvalid):
		writeError(w, r, http.StatusUnprocessableEntity, errCodeTemplateRenderPayloadInvalid, err.Error(), nil)
	case errors.Is(err, domain.ErrModeInvalid):
		writeError(w, r, http.StatusUnprocessableEntity, errCodeDeliveryModeInvalid, err.Error(), nil)
	case errors.Is(err, domain.ErrRawBodyRequired):
		writeError(w, r, http.StatusUnprocessableEntity, errCodeDeliveryRawBodyRequired, err.Error(), nil)
	case errors.Is(err, domain.ErrAttachmentTooLarge):
		writeError(w, r, http.StatusUnprocessableEntity, errCodeDeliveryAttachmentTooLarge, err.Error(), nil)
	case errors.Is(err, domain.ErrDuplicateRecipient):
		writeError(w, r, http.StatusUnprocessableEntity, errCodeDeliveryDuplicateRecipient, err.Error(), nil)
	case errors.Is(err, domain.ErrSubjectRequired):
		writeError(w, r, http.StatusUnprocessableEntity, errCodeDeliverySubjectRequired, err.Error(), nil)
	case errors.Is(err, domain.ErrAttachmentNotSupported):
		writeError(w, r, http.StatusUnprocessableEntity, errCodeDeliveryAttachmentNotSupported, err.Error(), nil)
	case errors.Is(err, domain.ErrAttachmentStorageFailed):
		writeError(w, r, http.StatusInternalServerError, errCodeDeliveryAttachmentStorageFailed, err.Error(), nil)
	case errors.Is(err, domain.ErrObjectStorageDisabled):
		writeError(w, r, http.StatusUnprocessableEntity, errCodeDeliveryObjectStorageDisabled, err.Error(), nil)
	case errors.Is(err, domain.ErrAPIKeyQuotaExceeded):
		writeError(w, r, http.StatusTooManyRequests, errCodeDeliveryQuotaExceeded, err.Error(), nil)
	case errors.Is(err, domain.ErrTemporarilyUnavailable):
		writeError(w, r, http.StatusServiceUnavailable, errCodeDeliveryTemporarilyUnavailable, err.Error(), nil)
	default:
		writeInternalError(w, r)
	}
}
