package app

import (
	"context"
	"errors"
	"time"

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

type HandleProviderEventInput struct {
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

type HandleProviderEventResult struct {
	Handled            bool
	Ignored            bool
	MessageID          string
	WorkspaceID        string
	PreviousStatus     string
	NewStatus          string
	SuppressionCreated bool
	SuppressionEntryID string
}

type providerEventClassifier struct{}

func (c *providerEventClassifier) ClassifyEventType(eventType string) (string, bool) {
	switch eventType {
	case "delivered":
		return domain.MessageStatusDelivered, true
	case "bounced":
		return domain.MessageStatusBounced, true
	case "complained":
		return domain.MessageStatusComplained, true
	case "delayed":
		return domain.MessageStatusDelayed, true
	case "rejected":
		return domain.MessageStatusFailed, true
	case "accepted", "opened", "clicked", "unsubscribed", "rendering_failed":
		return "", false
	default:
		return "", false
	}
}

func (c *providerEventClassifier) CanTransition(currentStatus, targetStatus string) bool {
	if currentStatus == targetStatus {
		return true
	}
	switch currentStatus {
	case domain.MessageStatusBounced, domain.MessageStatusFailed,
		domain.MessageStatusCancelled, domain.MessageStatusDLQ:
		return false
	case domain.MessageStatusComplained:
		return targetStatus == domain.MessageStatusComplained
	case domain.MessageStatusDelivered:
		return targetStatus == domain.MessageStatusDelivered || targetStatus == domain.MessageStatusComplained
	case domain.MessageStatusDelayed:
		return targetStatus != domain.MessageStatusAccepted
	case domain.MessageStatusAccepted:
		return true
	case domain.MessageStatusProcessing:
		return targetStatus != domain.MessageStatusAccepted
	case domain.MessageStatusQueued:
		return targetStatus != domain.MessageStatusDelivered && targetStatus != domain.MessageStatusComplained
	default:
		return false
	}
}

func (s *Service) HandleProviderEvent(ctx context.Context, input HandleProviderEventInput) (*HandleProviderEventResult, error) {
	log := s.log.With(
		"usecase", "handle_provider_event",
		"event_id", input.EventID,
		"normalized_event_id", input.NormalizedEventID,
		"workspace_id", input.WorkspaceID,
		"message_id", input.MessageID,
		"provider", input.Provider,
		"event_type", input.EventType,
	)

	if input.EventID == "" || input.Provider == "" || input.EventType == "" {
		return nil, &NonRetryableError{Err: domain.ErrPayloadInvalid}
	}
	if input.OccurredAt.IsZero() || input.ReceivedAt.IsZero() {
		return nil, &NonRetryableError{Err: domain.ErrPayloadInvalid}
	}

	var classifier providerEventClassifier
	targetStatus, recognized := classifier.ClassifyEventType(input.EventType)
	if !recognized {
		log.Info("event type not recognized for delivery state mutation, ignoring")
		return &HandleProviderEventResult{Ignored: true}, nil
	}

	var message *domain.Message
	var resolveErr error

	if input.WorkspaceID != "" && input.MessageID != "" {
		message, resolveErr = s.messagesRead.FindByID(ctx, input.WorkspaceID, input.MessageID)
	} else if input.Provider != "" && input.ProviderMessageID != "" {
		message, resolveErr = s.messagesRead.FindByProviderMessageID(ctx, input.Provider, input.ProviderMessageID)
		if message != nil && input.WorkspaceID != "" && message.WorkspaceID != input.WorkspaceID {
			log.Warn("message found by provider_message_id belongs to different workspace, skipping",
				"found_workspace_id", message.WorkspaceID,
			)
			return &HandleProviderEventResult{Ignored: true}, nil
		}
	}

	if resolveErr != nil {
		if errors.Is(resolveErr, domain.ErrMessageNotFound) {
			log.Info("message not found, ignoring event")
			return &HandleProviderEventResult{Ignored: true}, nil
		}
		log.Error("failed to resolve message", "error", resolveErr)
		return nil, resolveErr
	}
	if message == nil {
		log.Info("no message resolved, ignoring event")
		return &HandleProviderEventResult{Ignored: true}, nil
	}

	previousStatus := message.Status

	if !classifier.CanTransition(message.Status, targetStatus) {
		log.Info("status transition not allowed",
			"current_status", message.Status,
			"target_status", targetStatus,
		)
		return &HandleProviderEventResult{
			Handled:        true,
			MessageID:      message.ID,
			WorkspaceID:    message.WorkspaceID,
			PreviousStatus: previousStatus,
			NewStatus:      message.Status,
		}, nil
	}

	if targetStatus == message.Status {
		log.Info("message already in target status, no-op")
		return &HandleProviderEventResult{
			Handled:        true,
			MessageID:      message.ID,
			WorkspaceID:    message.WorkspaceID,
			PreviousStatus: previousStatus,
			NewStatus:      message.Status,
		}, nil
	}

	var result HandleProviderEventResult

	if err := s.txManager.RunInTransaction(ctx, func(txCtx context.Context) error {
		now := time.Now().UTC()

		// Re-read with FOR UPDATE inside the transaction to guard against concurrent
		// status changes. This serializes concurrent provider events for the same
		// message so that the CanTransition check sees the latest committed state.
		currentMessage, err := s.messagesRead.FindByIDForUpdate(txCtx, message.WorkspaceID, message.ID)
		if err != nil {
			log.Error("failed to re-read message inside transaction", "error", err)
			return err
		}
		if !classifier.CanTransition(currentMessage.Status, targetStatus) {
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

			// For delayed, we just update the message without publishing a delivery event
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

		if err := s.messagesWrite.Update(txCtx, updated); err != nil {
			log.Error("failed to update message status", "error", err)
			return err
		}

		if suppressionInput != nil && s.recipientSuppressor != nil {
			supResult, supErr := s.recipientSuppressor.SuppressFromSignal(txCtx, *suppressionInput)
			if supErr != nil {
				log.Warn("suppression creation failed, message update will proceed", "error", supErr)
			} else {
				result.SuppressionCreated = supResult.Created
				result.SuppressionEntryID = supResult.EntryID
			}
		}

		if outboxPayload != nil && outboxEventType != "" {
			eventID := mustNewID(s.idGen)

			envelope, err := events.NewEnvelope(events.NewEnvelopeOptions{
				EventID:       eventID,
				EventType:     outboxEventType,
				EventVersion:  1,
				AggregateType: "message",
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

			if err := s.outboxWriter.Save(txCtx, ports.OutboxEvent{
				ID:            eventID,
				AggregateType: "message",
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
				supEventID := mustNewID(s.idGen)
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
					AggregateType: "suppression_entry",
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

				if err := s.outboxWriter.Save(txCtx, ports.OutboxEvent{
					ID:            supEventID,
					AggregateType: "suppression_entry",
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
