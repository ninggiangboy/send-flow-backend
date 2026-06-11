package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/mail"
	"strings"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/contracts"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/ports"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/events"
)

const (
	maxIdempotencyKeyLength = 255
	maxRecipientNameLength  = 200
	maxMetadataSize         = 16 * 1024
	maxTemplateDataSize     = 64 * 1024
	maxTagsCount            = 20
	maxTagLength            = 64
)

type AcceptTransactionalSendInput struct {
	WorkspaceID       string
	APIKeyID          string
	IdempotencyKey    string
	RecipientEmail    string
	RecipientName     string
	SenderDomainID    string
	TemplateID        string
	TemplateVersionID string
	TemplateData      map[string]any
	Metadata          map[string]any
	Tags              []string
	Now               time.Time
}

type AcceptTransactionalSendResult struct {
	MessageID  string
	RequestID  string
	Status     string
	AcceptedAt time.Time
}

type canonicalPayload struct {
	To                []recipientPayload `json:"to"`
	SenderDomainID    string             `json:"sender_domain_id"`
	TemplateID        string             `json:"template_id"`
	TemplateVersionID string             `json:"template_version_id,omitempty"`
	TemplateData      map[string]any     `json:"template_data,omitempty"`
	Metadata          map[string]any     `json:"metadata,omitempty"`
	Tags              []string           `json:"tags,omitempty"`
	MessageType       string             `json:"message_type"`
}

type recipientPayload struct {
	Email string `json:"email"`
	Name  string `json:"name,omitempty"`
}

func (s *Service) AcceptTransactionalSend(ctx context.Context, input AcceptTransactionalSendInput) (*AcceptTransactionalSendResult, error) {
	log := s.log.With("usecase", "accept_transactional_send", "workspace_id", input.WorkspaceID)

	if err := s.validateSendInput(&input); err != nil {
		log.Warn("transactional send validation failed", "error", err)
		return nil, err
	}

	recipientEmail := normalizeEmail(input.RecipientEmail)

	canonical := canonicalPayload{
		To: []recipientPayload{{
			Email: recipientEmail,
			Name:  strings.TrimSpace(input.RecipientName),
		}},
		SenderDomainID:    strings.TrimSpace(input.SenderDomainID),
		TemplateID:        strings.TrimSpace(input.TemplateID),
		TemplateVersionID: strings.TrimSpace(input.TemplateVersionID),
		TemplateData:      input.TemplateData,
		Metadata:          input.Metadata,
		Tags:              cleanTags(input.Tags),
		MessageType:       "transactional",
	}

	canonicalJSON, err := json.Marshal(canonical)
	if err != nil {
		return nil, domain.ErrRequestBodyInvalid
	}

	inputKey := strings.TrimSpace(input.IdempotencyKey)

	if inputKey != "" {
		result, err := s.handleIdempotency(ctx, input.WorkspaceID, inputKey, canonicalJSON)
		if err != nil {
			return nil, err
		}
		if result != nil {
			return result, nil
		}
	}

	if err := s.checkSenderReadiness(ctx, input.WorkspaceID, input.SenderDomainID); err != nil {
		log.Warn("sender readiness check failed", "sender_domain_id", input.SenderDomainID, "error", err)
		return nil, err
	}

	if err := s.checkTemplate(ctx, input.WorkspaceID, input.TemplateID, input.TemplateVersionID, input.TemplateData); err != nil {
		log.Warn("template check failed", "template_id", input.TemplateID, "error", err)
		return nil, err
	}

	if err := s.checkSuppression(ctx, input.WorkspaceID, recipientEmail); err != nil {
		log.Warn("suppression check failed", "error", err)
		return nil, err
	}

	errUniqueViolation := errors.New("unique violation recovery needed")
	var result *AcceptTransactionalSendResult
	if err := s.txManager.WithinTx(ctx, func(txCtx context.Context) error {
		requestID, err := s.idGen()
		if err != nil {
			log.Error("failed to generate request ID", "error", err)
			return err
		}
		messageID, err := s.idGen()
		if err != nil {
			log.Error("failed to generate message ID", "error", err)
			return err
		}
		now := input.Now

		var idempotencyKey *string
		if inputKey != "" {
			idempotencyKey = &inputKey
		}
		txReq := domain.TransactionalSendRequest{
			ID:             requestID,
			WorkspaceID:    input.WorkspaceID,
			IdempotencyKey: idempotencyKey,
			Status:         domain.TxRequestStatusAccepted,
			RequestPayload: canonicalJSON,
			CreatedAt:      now,
			UpdatedAt:      now,
		}

		recipientSnapshot := domain.RecipientSnapshot{
			Email:           input.RecipientEmail,
			EmailNormalized: recipientEmail,
			FirstName:       strings.TrimSpace(input.RecipientName),
			Tags:            canonical.Tags,
			Attributes:      input.Metadata,
			TemplateData:    input.TemplateData,
		}

		msg := domain.Message{
			ID:                       messageID,
			WorkspaceID:              input.WorkspaceID,
			TransactionalRequestID:   requestID,
			RecipientEmailNormalized: recipientEmail,
			RecipientSnapshot:        recipientSnapshot,
			TemplateID:               input.TemplateID,
			TemplateVersionID:        canonical.TemplateVersionID,
			SenderDomainID:           input.SenderDomainID,
			MessageType:              domain.MessageTypeTransactional,
			SourceType:               domain.MessageSourceTransactional,
			Status:                   domain.MessageStatusQueued,
			QueuedAt:                 &now,
			CreatedAt:                now,
			UpdatedAt:                now,
		}

		if err := s.txRequestsWrite.Create(txCtx, txReq); err != nil {
			if errors.Is(err, domain.ErrIdempotencyKeyConflict) {
				return errUniqueViolation
			}
			return err
		}

		insertedIDs, err := s.messagesWrite.CreateMany(txCtx, []domain.Message{msg})
		if err != nil {
			return err
		}
		if len(insertedIDs) == 0 {
			return domain.ErrTemporarilyUnavailable
		}

		eventID, err := s.idGen()
		if err != nil {
			log.Error("failed to generate event ID", "error", err)
			return err
		}
		payload := contracts.MessageQueuedPayload{
			MessageID:              messageID,
			WorkspaceID:            input.WorkspaceID,
			TransactionalRequestID: requestID,
			TemplateID:             input.TemplateID,
			TemplateVersionID:      canonical.TemplateVersionID,
			SenderDomainID:         input.SenderDomainID,
			MessageType:            domain.MessageTypeTransactional,
			SourceType:             domain.MessageSourceTransactional,
		}
		envelope, err := events.NewEnvelope(events.NewEnvelopeOptions{
			EventID:       eventID,
			EventType:     contracts.EventDeliveryMessageQueuedV1,
			EventVersion:  1,
			AggregateType: "message",
			AggregateID:   messageID,
			WorkspaceID:   input.WorkspaceID,
			OccurredAt:    now,
		}, payload)
		if err != nil {
			return err
		}
		payloadBytes, err := events.Marshal(envelope)
		if err != nil {
			return err
		}
		if err := s.outboxWriter.Save(txCtx, ports.OutboxEvent{
			ID:            envelope.EventID,
			AggregateType: "message",
			AggregateID:   messageID,
			EventType:     contracts.EventDeliveryMessageQueuedV1,
			Payload:       payloadBytes,
			WorkspaceID:   input.WorkspaceID,
			OccurredAt:    now,
		}); err != nil {
			return err
		}

		acceptedEventID, err := s.idGen()
		if err != nil {
			log.Error("failed to generate accepted event ID", "error", err)
			return err
		}
		acceptedPayload := contracts.MessageQueuedPayload{
			MessageID:              messageID,
			WorkspaceID:            input.WorkspaceID,
			TransactionalRequestID: requestID,
			TemplateID:             input.TemplateID,
			TemplateVersionID:      canonical.TemplateVersionID,
			SenderDomainID:         input.SenderDomainID,
			MessageType:            domain.MessageTypeTransactional,
			SourceType:             domain.MessageSourceTransactional,
		}
		acceptedEnvelope, err := events.NewEnvelope(events.NewEnvelopeOptions{
			EventID:       acceptedEventID,
			EventType:     contracts.EventDeliveryTransactionalSendAcceptedV1,
			EventVersion:  1,
			AggregateType: "transactional_send_request",
			AggregateID:   requestID,
			WorkspaceID:   input.WorkspaceID,
			OccurredAt:    now,
		}, acceptedPayload)
		if err != nil {
			return err
		}
		acceptedPayloadBytes, err := events.Marshal(acceptedEnvelope)
		if err != nil {
			return err
		}
		if err := s.outboxWriter.Save(txCtx, ports.OutboxEvent{
			ID:            acceptedEnvelope.EventID,
			AggregateType: "transactional_send_request",
			AggregateID:   requestID,
			EventType:     contracts.EventDeliveryTransactionalSendAcceptedV1,
			Payload:       acceptedPayloadBytes,
			WorkspaceID:   input.WorkspaceID,
			OccurredAt:    now,
		}); err != nil {
			return err
		}

		result = &AcceptTransactionalSendResult{
			MessageID:  messageID,
			RequestID:  requestID,
			Status:     domain.TxRequestStatusAccepted,
			AcceptedAt: now,
		}
		return nil
	}); err != nil {
		if errors.Is(err, errUniqueViolation) {
			existingReq, lookupErr := s.txRequestsRead.FindByIdempotencyKey(ctx, input.WorkspaceID, inputKey)
			if lookupErr != nil {
				log.Error("failed to lookup conflicting request on unique violation recovery", "idempotency_key", inputKey, "error", lookupErr)
				return nil, domain.ErrTemporarilyUnavailable
			}
			if !bytes.Equal(existingReq.RequestPayload, canonicalJSON) {
				return nil, domain.ErrIdempotencyKeyConflict
			}
			existingMsg, lookupErr := s.messagesRead.FindByTransactionalRequestID(ctx, input.WorkspaceID, existingReq.ID)
			if lookupErr != nil {
				log.Error("found request but no message on unique violation recovery", "request_id", existingReq.ID, "error", lookupErr)
				return nil, domain.ErrTemporarilyUnavailable
			}
			result = &AcceptTransactionalSendResult{
				MessageID:  existingMsg.ID,
				RequestID:  existingReq.ID,
				Status:     existingReq.Status,
				AcceptedAt: existingReq.CreatedAt,
			}
		} else if errors.Is(err, domain.ErrIdempotencyKeyConflict) {
			return nil, err
		} else {
			log.Error("transactional send transaction failed", "error", err)
			return nil, err
		}
	}

	log.Info("transactional send accepted",
		"request_id", result.RequestID,
		"message_id", result.MessageID,
	)

	return result, nil
}

func (s *Service) validateSendInput(input *AcceptTransactionalSendInput) error {
	if input.WorkspaceID == "" {
		return domain.ErrRequestBodyInvalid
	}

	input.RecipientEmail = strings.TrimSpace(input.RecipientEmail)
	if input.RecipientEmail == "" {
		return domain.ErrRecipientInvalid
	}
	if _, err := mail.ParseAddress(input.RecipientEmail); err != nil {
		return domain.ErrRecipientInvalid
	}

	input.RecipientName = strings.TrimSpace(input.RecipientName)
	if len(input.RecipientName) > maxRecipientNameLength {
		input.RecipientName = input.RecipientName[:maxRecipientNameLength]
	}

	input.SenderDomainID = strings.TrimSpace(input.SenderDomainID)
	if input.SenderDomainID == "" {
		return domain.ErrRequestBodyInvalid
	}

	input.TemplateID = strings.TrimSpace(input.TemplateID)
	if input.TemplateID == "" {
		return domain.ErrRequestBodyInvalid
	}

	input.TemplateVersionID = strings.TrimSpace(input.TemplateVersionID)

	if len(input.Tags) > maxTagsCount {
		input.Tags = input.Tags[:maxTagsCount]
	}
	for i, tag := range input.Tags {
		if len(tag) > maxTagLength {
			input.Tags[i] = tag[:maxTagLength]
		}
	}

	if input.Metadata != nil {
		data, err := json.Marshal(input.Metadata)
		if err != nil {
			return domain.ErrRequestBodyInvalid
		}
		if len(data) > maxMetadataSize {
			return domain.ErrRequestBodyInvalid
		}
	}

	if input.TemplateData != nil {
		data, err := json.Marshal(input.TemplateData)
		if err != nil {
			return domain.ErrRequestBodyInvalid
		}
		if len(data) > maxTemplateDataSize {
			return domain.ErrRequestBodyInvalid
		}
	}

	if input.Now.IsZero() {
		input.Now = time.Now().UTC()
	}

	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	if len(input.IdempotencyKey) > maxIdempotencyKeyLength {
		return domain.ErrRequestBodyInvalid
	}

	return nil
}

func (s *Service) handleIdempotency(ctx context.Context, workspaceID, idempotencyKey string, canonicalJSON []byte) (*AcceptTransactionalSendResult, error) {
	existingReq, err := s.txRequestsRead.FindByIdempotencyKey(ctx, workspaceID, idempotencyKey)
	if err != nil {
		if errors.Is(err, domain.ErrTransactionalRequestNotFound) {
			return nil, nil
		}
		return nil, domain.ErrTemporarilyUnavailable
	}

	if !bytes.Equal(existingReq.RequestPayload, canonicalJSON) {
		return nil, domain.ErrIdempotencyKeyConflict
	}

	existingMsg, err := s.messagesRead.FindByTransactionalRequestID(ctx, workspaceID, existingReq.ID)
	if err != nil {
		s.log.Error("found request but no message for idempotent retry",
			"request_id", existingReq.ID,
			"workspace_id", workspaceID,
			"error", err,
		)
		return nil, domain.ErrTemporarilyUnavailable
	}

	return &AcceptTransactionalSendResult{
		MessageID:  existingMsg.ID,
		RequestID:  existingReq.ID,
		Status:     existingReq.Status,
		AcceptedAt: existingReq.CreatedAt,
	}, nil
}

func (s *Service) checkSenderReadiness(ctx context.Context, workspaceID, senderDomainID string) error {
	readiness, err := s.senderChecker.GetSenderReadiness(ctx, workspaceID, senderDomainID)
	if err != nil {
		if errors.Is(err, domain.ErrSenderDomainNotFound) {
			return err
		}
		return domain.ErrTemporarilyUnavailable
	}
	if readiness == nil || !readiness.Ready {
		return domain.ErrSenderDomainNotVerified
	}
	return nil
}

func (s *Service) checkTemplate(ctx context.Context, workspaceID, templateID, templateVersionID string, templateData map[string]any) error {
	_, err := s.contentRenderer.RenderForMessage(ctx, workspaceID, templateID, templateVersionID, templateData)
	if err != nil {
		if errors.Is(err, domain.ErrTemplateNotFound) || errors.Is(err, domain.ErrTemplateRenderPayloadInvalid) {
			return err
		}
		return domain.ErrTemporarilyUnavailable
	}
	return nil
}

func (s *Service) checkSuppression(ctx context.Context, workspaceID, emailNormalized string) error {
	decision, err := s.suppressionChecker.CheckSuppression(ctx, workspaceID, emailNormalized, "workspace")
	if err != nil {
		return domain.ErrTemporarilyUnavailable
	}
	if decision != nil && decision.Suppressed {
		return domain.ErrSuppressedRecipient
	}
	return nil
}

type GetTransactionalMessageInput struct {
	WorkspaceID string
	MessageID   string
}

type GetTransactionalMessageResult struct {
	MessageID         string     `json:"message_id"`
	Status            string     `json:"status"`
	Provider          string     `json:"provider,omitempty"`
	ProviderMessageID string     `json:"provider_message_id,omitempty"`
	LastUpdatedAt     *time.Time `json:"last_updated_at"`
}

func (s *Service) GetTransactionalMessage(ctx context.Context, input GetTransactionalMessageInput) (*GetTransactionalMessageResult, error) {
	if input.WorkspaceID == "" || input.MessageID == "" {
		return nil, domain.ErrPayloadInvalid
	}

	msg, err := s.messagesRead.FindByID(ctx, input.WorkspaceID, input.MessageID)
	if err != nil {
		if errors.Is(err, domain.ErrMessageNotFound) {
			return nil, err
		}
		s.log.Error("failed to find transactional message", "workspace_id", input.WorkspaceID, "message_id", input.MessageID, "error", err)
		return nil, err
	}

	if msg.SourceType != domain.MessageSourceTransactional {
		return nil, domain.ErrMessageNotFound
	}

	ts := msg.UpdatedAt
	if msg.AcceptedAt != nil {
		ts = *msg.AcceptedAt
	}

	return &GetTransactionalMessageResult{
		MessageID:         msg.ID,
		Status:            msg.Status,
		Provider:          msg.Provider,
		ProviderMessageID: msg.ProviderMessageID,
		LastUpdatedAt:     &ts,
	}, nil
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func cleanTags(tags []string) []string {
	if len(tags) == 0 {
		return nil
	}
	cleaned := make([]string, 0, len(tags))
	for _, t := range tags {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		if len(t) > maxTagLength {
			t = t[:maxTagLength]
		}
		cleaned = append(cleaned, t)
	}
	if len(cleaned) > maxTagsCount {
		cleaned = cleaned[:maxTagsCount]
	}
	if len(cleaned) == 0 {
		return nil
	}
	return cleaned
}
