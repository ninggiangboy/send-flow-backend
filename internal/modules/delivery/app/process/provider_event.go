package process

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/app/shared"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/contracts"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/ports"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/events"
)

type SuppressFromSignalResult struct {
	EntryID string
	Created bool
}

type RecipientSuppressor interface {
	SuppressFromSignal(ctx context.Context, input SuppressFromSignalInput) (*SuppressFromSignalResult, error)
}

type SuppressFromSignalInput struct {
	WorkspaceID     string
	Email           string
	EmailNormalized string
	Scope           string
	Reason          string
	Source          string
	SourceEventID   string
	Note            string
	Now             time.Time
}

type ProviderEventInput struct {
	EventID           string
	NormalizedEventID string
	RawEventID        string
	WorkspaceID       string
	MessageID         string
	Provider          string
	ProviderEventID   string
	ProviderMessageID string
	EventType         string
	OccurredAt        time.Time
	ReceivedAt        time.Time
}

type ProviderEventResult struct {
	Handled            bool
	Ignored            bool
	MessageID          string
	WorkspaceID        string
	PreviousStatus     string
	NewStatus          string
	SuppressionCreated bool
	SuppressionEntryID string
}

type ProviderEventHandler struct {
	messagesWrite       ports.MessageWriteRepository
	txRequestsWrite     ports.TransactionalRequestWriteRepository
	recipientSuppressor RecipientSuppressor
	outboxWriter        ports.OutboxWriter
	txManager           ports.UnitOfWork
	idGen               func() (string, error)
	log                 *slog.Logger
	eventRepo           ports.MessageEventRepository
}

func NewProviderEventHandler(
	messagesWrite ports.MessageWriteRepository,
	txRequestsWrite ports.TransactionalRequestWriteRepository,
	recipientSuppressor RecipientSuppressor,
	outboxWriter ports.OutboxWriter,
	txManager ports.UnitOfWork,
	idGen func() (string, error),
	logger *slog.Logger,
	eventRepo ports.MessageEventRepository,
) *ProviderEventHandler {
	return &ProviderEventHandler{
		messagesWrite:       messagesWrite,
		txRequestsWrite:     txRequestsWrite,
		recipientSuppressor: recipientSuppressor,
		outboxWriter:        outboxWriter,
		txManager:           txManager,
		idGen:               idGen,
		log:                 logger.With("usecase", "handle_provider_event"),
		eventRepo:           eventRepo,
	}
}

func (h *ProviderEventHandler) writeEvent(ctx context.Context, msg domain.Message, eventType, reasonCode, reasonMessage string, now time.Time) {
	if h.eventRepo == nil {
		return
	}
	eventID, err := h.idGen()
	if err != nil {
		h.log.Error("failed to generate event id", "error", err)
		return
	}
	evt := domain.MessageEvent{
		ID:                     eventID,
		WorkspaceID:            msg.WorkspaceID,
		MessageID:              msg.ID,
		TransactionalRequestID: msg.TransactionalRequestID,
		EventType:              eventType,
		Status:                 msg.Status,
		ReasonCode:             reasonCode,
		ReasonMessage:          reasonMessage,
		OccurredAt:             now,
		CreatedAt:              now,
	}
	if err := h.eventRepo.Create(ctx, evt); err != nil {
		h.log.Error("failed to write message event",
			"message_id", msg.ID,
			"event_type", eventType,
			"error", err,
		)
	}
}

func (h *ProviderEventHandler) writeMessageEvent(ctx context.Context, msg domain.Message, targetStatus string, now time.Time) {
	var eventType, reasonCode string
	switch targetStatus {
	case domain.MessageStatusDelivered:
		eventType = domain.MessageEventDelivered
	case domain.MessageStatusBounced:
		eventType = domain.MessageEventBounced
		reasonCode = "provider_bounce"
	case domain.MessageStatusComplained:
		eventType = domain.MessageEventComplained
		reasonCode = "provider_complaint"
	case domain.MessageStatusDelayed:
		eventType = domain.MessageStatusDelayed
		reasonCode = "provider_delayed"
	case domain.MessageStatusFailed:
		eventType = domain.MessageEventFailed
		reasonCode = "provider_rejected"
	default:
		return
	}
	h.writeEvent(ctx, msg, eventType, reasonCode, "", now)
}

func (h *ProviderEventHandler) Execute(ctx context.Context, input ProviderEventInput) (*ProviderEventResult, error) {
	log := h.log.With(
		"event_id", input.EventID,
		"normalized_event_id", input.NormalizedEventID,
		"workspace_id", input.WorkspaceID,
		"message_id", input.MessageID,
		"provider", input.Provider,
		"event_type", input.EventType,
	)

	if input.EventID == "" || input.Provider == "" || input.EventType == "" {
		return nil, &shared.NonRetryableError{Err: domain.ErrPayloadInvalid}
	}
	if input.OccurredAt.IsZero() || input.ReceivedAt.IsZero() {
		return nil, &shared.NonRetryableError{Err: domain.ErrPayloadInvalid}
	}

	targetStatus, recognized := domain.ClassifyProviderEvent(input.EventType)
	if !recognized {
		log.Info("event type not recognized for delivery state mutation, ignoring")
		return &ProviderEventResult{Ignored: true}, nil
	}

	var message *domain.Message
	var resolveErr error

	if input.WorkspaceID != "" && input.MessageID != "" {
		message, resolveErr = h.messagesWrite.FindByID(ctx, input.WorkspaceID, input.MessageID)
	} else if input.Provider != "" && input.ProviderMessageID != "" {
		message, resolveErr = h.messagesWrite.FindByProviderMessageID(ctx, input.Provider, input.ProviderMessageID)
		if message != nil && input.WorkspaceID != "" && message.WorkspaceID != input.WorkspaceID {
			log.Warn("message found by provider_message_id belongs to different workspace, skipping",
				"found_workspace_id", message.WorkspaceID,
			)
			return &ProviderEventResult{Ignored: true}, nil
		}
	}

	if resolveErr != nil {
		if errors.Is(resolveErr, domain.ErrMessageNotFound) {
			log.Info("message not found, ignoring event")
			return &ProviderEventResult{Ignored: true}, nil
		}
		log.Error("failed to resolve message", "error", resolveErr)
		return nil, resolveErr
	}
	if message == nil {
		log.Info("no message resolved, ignoring event")
		return &ProviderEventResult{Ignored: true}, nil
	}

	previousStatus := message.Status

	if !domain.CanTransitionToStatus(message.Status, targetStatus) {
		log.Info("status transition not allowed",
			"current_status", message.Status,
			"target_status", targetStatus,
		)
		return &ProviderEventResult{
			Handled:        true,
			MessageID:      message.ID,
			WorkspaceID:    message.WorkspaceID,
			PreviousStatus: previousStatus,
			NewStatus:      message.Status,
		}, nil
	}

	if targetStatus == message.Status {
		log.Info("message already in target status, no-op")
		return &ProviderEventResult{
			Handled:        true,
			MessageID:      message.ID,
			WorkspaceID:    message.WorkspaceID,
			PreviousStatus: previousStatus,
			NewStatus:      message.Status,
		}, nil
	}

	var result ProviderEventResult

	if err := h.txManager.WithinTx(ctx, func(txCtx context.Context) error {
		now := time.Now().UTC()

		currentMessage, err := h.messagesWrite.FindByIDForUpdate(txCtx, message.WorkspaceID, message.ID)
		if err != nil {
			if errors.Is(err, domain.ErrMessageNotFound) {
				log.Warn("message not found during transactional re-read, using resolved snapshot")
				currentMessage = message
			} else {
				log.Error("failed to re-read message inside transaction", "error", err)
				return err
			}
		}
		if currentMessage == nil {
			log.Error("failed to re-read message inside transaction", "error", err)
			return domain.ErrMessageNotFound
		}
		if !domain.CanTransitionToStatus(currentMessage.Status, targetStatus) {
			log.Warn("concurrent status change prevents transition, skipping",
				"current_status", currentMessage.Status,
				"target_status", targetStatus,
			)
			return nil
		}
		if currentMessage.Status == targetStatus {
			return nil
		}

		updated := *currentMessage
		updated.Status = targetStatus
		updated.UpdatedAt = now

		var outboxPayload any
		var outboxEventType string
		var suppressionInput *SuppressFromSignalInput

		switch targetStatus {
		case domain.MessageStatusDelivered:
			updated.DeliveredAt = &input.OccurredAt
			updated.LastErrorClass = ""
			updated.LastErrorMessage = ""

			outboxPayload = contracts.MessageDeliveredPayload{
				MessageID:         updated.ID,
				WorkspaceID:       updated.WorkspaceID,
				CampaignID:        updated.CampaignID,
				Provider:          updated.Provider,
				ProviderMessageID: updated.ProviderMessageID,
				ProviderEventID:   input.ProviderEventID,
				NormalizedEventID: input.NormalizedEventID,
				OccurredAt:        input.OccurredAt.Format(time.RFC3339),
				ReceivedAt:        input.ReceivedAt.Format(time.RFC3339),
			}
			outboxEventType = contracts.EventDeliveryMessageDeliveredV1

		case domain.MessageStatusBounced:
			updated.BouncedAt = &input.OccurredAt
			updated.LastErrorClass = "provider_bounce"
			updated.LastErrorMessage = ""

			suppressionInput = &SuppressFromSignalInput{
				WorkspaceID:     updated.WorkspaceID,
				Email:           updated.RecipientSnapshot.Email,
				EmailNormalized: updated.RecipientEmailNormalized,
				Scope:           "workspace",
				Reason:          "bounce",
				Source:          "provider_event",
				SourceEventID:   input.NormalizedEventID,
				Note:            "auto-suppressed from provider bounce",
				Now:             now,
			}

			outboxPayload = contracts.MessageBouncedPayload{
				MessageID:         updated.ID,
				WorkspaceID:       updated.WorkspaceID,
				CampaignID:        updated.CampaignID,
				Provider:          updated.Provider,
				ProviderMessageID: updated.ProviderMessageID,
				ProviderEventID:   input.ProviderEventID,
				NormalizedEventID: input.NormalizedEventID,
				OccurredAt:        input.OccurredAt.Format(time.RFC3339),
				ReceivedAt:        input.ReceivedAt.Format(time.RFC3339),
			}
			outboxEventType = contracts.EventDeliveryMessageBouncedV1

		case domain.MessageStatusComplained:
			updated.ComplainedAt = &input.OccurredAt
			updated.LastErrorClass = "provider_complaint"
			updated.LastErrorMessage = ""

			suppressionInput = &SuppressFromSignalInput{
				WorkspaceID:     updated.WorkspaceID,
				Email:           updated.RecipientSnapshot.Email,
				EmailNormalized: updated.RecipientEmailNormalized,
				Scope:           "workspace",
				Reason:          "complaint",
				Source:          "provider_event",
				SourceEventID:   input.NormalizedEventID,
				Note:            "auto-suppressed from provider complaint",
				Now:             now,
			}

			outboxPayload = contracts.MessageComplainedPayload{
				MessageID:         updated.ID,
				WorkspaceID:       updated.WorkspaceID,
				CampaignID:        updated.CampaignID,
				Provider:          updated.Provider,
				ProviderMessageID: updated.ProviderMessageID,
				ProviderEventID:   input.ProviderEventID,
				NormalizedEventID: input.NormalizedEventID,
				OccurredAt:        input.OccurredAt.Format(time.RFC3339),
				ReceivedAt:        input.ReceivedAt.Format(time.RFC3339),
			}
			outboxEventType = contracts.EventDeliveryMessageComplainedV1

		case domain.MessageStatusDelayed:
			updated.LastErrorClass = "provider_delayed"
			updated.LastErrorMessage = ""

			outboxPayload = nil
			outboxEventType = ""

		case domain.MessageStatusFailed:
			updated.FailedAt = &input.OccurredAt
			updated.LastErrorClass = "provider_rejected"
			updated.LastErrorMessage = ""

			outboxPayload = nil
			outboxEventType = ""

		default:
			return nil
		}

		if err := h.messagesWrite.Update(txCtx, updated); err != nil {
			log.Error("failed to update message status", "error", err)
			return err
		}
		h.writeMessageEvent(txCtx, updated, targetStatus, now)

		// Update parent transactional request aggregate counters when
		// a recipient message reaches a terminal state.
		if updated.TransactionalRequestID != "" && updated.IsTerminal() {
			isSuccess := updated.Status == domain.MessageStatusDelivered
			if aggErr := h.txRequestsWrite.UpdateAggregates(txCtx, updated.WorkspaceID, updated.TransactionalRequestID, true, isSuccess, now); aggErr != nil {
				log.Warn("failed to update request aggregates", "error", aggErr)
			}
		}

		if suppressionInput != nil && h.recipientSuppressor != nil {
			supResult, supErr := h.recipientSuppressor.SuppressFromSignal(txCtx, *suppressionInput)
			if supErr != nil {
				log.Warn("suppression creation failed, message update will proceed", "error", supErr)
			} else {
				result.SuppressionCreated = supResult.Created
				result.SuppressionEntryID = supResult.EntryID
			}
		}

		if outboxPayload != nil && outboxEventType != "" {
			eventID, err := h.idGen()
			if err != nil {
				log.Error("failed to generate event ID", "error", err)
				return err
			}

			envelope, err := events.NewEnvelope(events.NewEnvelopeOptions{
				EventID:       eventID,
				EventType:     outboxEventType,
				EventVersion:  1,
				AggregateType: contracts.AggregateMessage,
				AggregateID:   updated.ID,
				WorkspaceID:   updated.WorkspaceID,
				OccurredAt:    now,
			}, outboxPayload)
			if err != nil {
				log.Error("failed to create outbox envelope", "error", err)
				return err
			}

			payloadBytes, err := events.Marshal(envelope)
			if err != nil {
				log.Error("failed to marshal outbox event", "error", err)
				return err
			}

			if err := h.outboxWriter.Save(txCtx, ports.OutboxEvent{
				ID:            eventID,
				AggregateType: contracts.AggregateMessage,
				AggregateID:   updated.ID,
				EventType:     outboxEventType,
				Payload:       payloadBytes,
				WorkspaceID:   updated.WorkspaceID,
				OccurredAt:    now,
			}); err != nil {
				log.Error("failed to save outbox event", "error", err)
				return err
			}

			if suppressionInput != nil && result.SuppressionCreated {
				supEventID, err := h.idGen()
				if err != nil {
					log.Error("failed to generate suppression event ID", "error", err)
					return err
				}
				supPayload := contracts.SuppressionRecipientSuppressedPayload{
					SuppressionID:   result.SuppressionEntryID,
					WorkspaceID:     updated.WorkspaceID,
					EmailNormalized: updated.RecipientEmailNormalized,
					Scope:           "workspace",
					Reason:          suppressionInput.Reason,
					Source:          suppressionInput.Source,
					SourceEventID:   suppressionInput.SourceEventID,
					CreatedAt:       now.Format(time.RFC3339),
				}

				supEnvelope, err := events.NewEnvelope(events.NewEnvelopeOptions{
					EventID:       supEventID,
					EventType:     contracts.EventSuppressionRecipientSuppressedV1,
					EventVersion:  1,
					AggregateType: contracts.AggregateSuppressionEntry,
					AggregateID:   result.SuppressionEntryID,
					WorkspaceID:   updated.WorkspaceID,
					OccurredAt:    now,
				}, supPayload)
				if err != nil {
					log.Error("failed to create suppression outbox envelope", "error", err)
					return err
				}

				supPayloadBytes, err := events.Marshal(supEnvelope)
				if err != nil {
					log.Error("failed to marshal suppression outbox event", "error", err)
					return err
				}

				if err := h.outboxWriter.Save(txCtx, ports.OutboxEvent{
					ID:            supEventID,
					AggregateType: contracts.AggregateSuppressionEntry,
					AggregateID:   result.SuppressionEntryID,
					EventType:     contracts.EventSuppressionRecipientSuppressedV1,
					Payload:       supPayloadBytes,
					WorkspaceID:   updated.WorkspaceID,
					OccurredAt:    now,
				}); err != nil {
					log.Error("failed to save suppression outbox event", "error", err)
					return err
				}
			}
		}

		return nil
	}); err != nil {
		log.Error("handle provider event transaction failed", "error", err)
		return nil, err
	}

	result.Handled = true
	result.MessageID = message.ID
	result.WorkspaceID = message.WorkspaceID
	result.PreviousStatus = previousStatus
	result.NewStatus = targetStatus

	log.Info("provider event handled",
		"previous_status", previousStatus,
		"new_status", targetStatus,
		"suppression_created", result.SuppressionCreated,
	)
	return &result, nil
}
