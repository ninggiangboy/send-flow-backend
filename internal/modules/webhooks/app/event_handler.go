package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/webhooks/domain"
	platformerrors "github.com/ninggiangboy/send-flow/backend/internal/platform/retryable"
)

type HandleSourceEventInput struct {
	RawPayload []byte
}

func (s *Service) HandleSourceEvent(ctx context.Context, input HandleSourceEventInput) error {
	if len(input.RawPayload) == 0 {
		return nil
	}

	var envelope struct {
		EventID     string          `json:"event_id"`
		EventType   string          `json:"event_type"`
		WorkspaceID string          `json:"workspace_id"`
		OccurredAt  time.Time       `json:"occurred_at"`
		Payload     json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(input.RawPayload, &envelope); err != nil {
		return &platformerrors.NonRetryableError{Err: fmt.Errorf("malformed envelope: %w", err)}
	}

	if envelope.EventType == "" || envelope.WorkspaceID == "" {
		return &platformerrors.NonRetryableError{Err: errors.New("event missing event_type or workspace_id")}
	}

	supported := domain.AllowedSubscriptionEvents[envelope.EventType]
	if !supported {
		return nil
	}

	log := s.log.With("usecase", "handle_source_event", "workspace_id", envelope.WorkspaceID, "event_type", envelope.EventType, "event_id", envelope.EventID)

	configs, err := s.configRead.ListSubscribed(ctx, envelope.WorkspaceID, envelope.EventType)
	if err != nil {
		log.Error("failed to list subscribed configs", "error", err)
		return err
	}

	now := s.clock()

	for _, cfg := range configs {
		deliveryID, err := s.idGen()
		if err != nil {
			return err
		}

		eventPayload := map[string]any{
			"id":           envelope.EventID,
			"type":         envelope.EventType,
			"workspace_id": envelope.WorkspaceID,
			"occurred_at":  envelope.OccurredAt.UTC().Format(time.RFC3339),
			"data":         json.RawMessage(envelope.Payload),
		}

		delivery := domain.WebhookDelivery{
			ID:               deliveryID,
			WorkspaceID:      envelope.WorkspaceID,
			WebhookID:        cfg.ID,
			SourceEventID:    envelope.EventID,
			SourceEventType:  envelope.EventType,
			Status:           domain.DeliveryStatusPending,
			TargetURL:        cfg.TargetURL,
			AttemptCount:     0,
			RequestHeaders:   map[string]string{},
			ResponseHeaders:  map[string]string{},
			EventPayloadJSON: eventPayload,
			CreatedAt:        now,
			UpdatedAt:        now,
		}

		if err := s.deliveryWrite.Create(ctx, delivery); err != nil {
			log.Error("failed to create delivery record", "error", err, "webhook_id", cfg.ID)
			continue
		}
	}

	log.Info("source event processed", "matched_configs", len(configs))
	return nil
}
