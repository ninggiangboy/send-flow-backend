package worker

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	analyticscontracts "github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/contracts"
	deliveryapp "github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/app"
	ingestioncontracts "github.com/ninggiangboy/send-flow/backend/internal/modules/ingestion/contracts"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/events"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/id"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/kafka"
)

type ProviderEventConsumer struct {
	name       string
	svc        *deliveryapp.Service
	log        *slog.Logger
	brokers    []string
	groupID    string
	markers    *ProcessedEventMarkers
	deadLetter *DeadLetterRepository
	idGen      func() (string, error)
	recordOp   OperationsEventRecorder
}

func (c *ProviderEventConsumer) SetOperationsRecorder(r OperationsEventRecorder) {
	c.recordOp = r
}

func NewProviderEventConsumer(svc *deliveryapp.Service, log *slog.Logger, brokers []string, groupID string, pool *pgxpool.Pool) *ProviderEventConsumer {
	c := &ProviderEventConsumer{
		name:    "delivery_provider_events",
		svc:     svc,
		log:     log.With("consumer", "delivery_provider_events"),
		brokers: brokers,
		groupID: groupID,
		idGen:   id.NewUUIDGenerator().New,
	}
	if pool != nil {
		c.markers = NewProcessedEventMarkers(pool)
		c.deadLetter = NewDeadLetterRepository(pool)
	}
	return c
}

func (c *ProviderEventConsumer) Name() string {
	return c.name
}

func (c *ProviderEventConsumer) Run(ctx context.Context) error {
	if len(c.brokers) == 0 {
		c.log.Info("kafka not configured, consumer disabled")
		<-ctx.Done()
		return nil
	}

	topic := events.TopicFromEventType(ingestioncontracts.EventProviderEventNormalizedV1)

	consumer, err := kafka.NewReaderConsumer(kafka.ReaderConsumerOptions{
		Brokers: c.brokers,
		Topic:   topic,
		GroupID: c.groupID,
	})
	if err != nil {
		if errors.Is(err, kafka.ErrDisabled) {
			c.log.Info("kafka disabled, consumer not starting")
			<-ctx.Done()
			return nil
		}
		return err
	}
	defer consumer.Close()

	c.log.Info("starting kafka consumer", "topic", topic, "group_id", c.groupID)

	return consumer.Consume(ctx, func(ctx context.Context, msg kafka.Message) error {
		eventID := msg.Headers["event_id"]
		if eventID == "" {
			c.log.Warn("received message without event_id header, skipping")
			return nil
		}

		if c.markers != nil {
			already, err := c.markers.WasProcessed(ctx, c.name, eventID)
			if err != nil {
				c.log.Error("failed to check processed marker", "event_id", eventID, "error", err)
				return err
			}
			if already {
				c.log.Debug("duplicate event, skipping", "event_id", eventID)
				return nil
			}
		}

		if err := c.HandleEvent(ctx, eventID, msg.Value); err != nil {
			var nonRetryable *deliveryapp.NonRetryableError
			if errors.As(err, &nonRetryable) {
				c.log.Warn("non-retryable error handling provider event",
					"event_id", eventID,
					"error", err,
				)
				wsID := workspaceIDFromMessage(msg.Headers, msg.Value)
				evType := eventTypeFromEnvelope(msg.Value)
				if c.deadLetter != nil {
					if dlErr := c.deadLetter.Save(ctx, DeadLetterRecord{
						ID:              mustNewID(c.idGen),
						WorkspaceID:     wsID,
						Source:          c.name,
						SourceEventType: evType,
						EventID:         eventID,
						Payload:         msg.Value,
						ErrorMessage:    err.Error(),
						Retryable:       false,
					}); dlErr != nil {
						c.log.Error("failed to save dead letter record, returning for retry",
							"event_id", eventID, "error", dlErr,
						)
						return dlErr
					}
				}
				if c.recordOp != nil && wsID != "" {
					now := time.Now()
					c.recordOp(ctx, c.name, evType, analyticscontracts.OperationTypeConsumerFailure, "failed", wsID, "non_retryable", c.name, "", now)
					c.recordOp(ctx, c.name, evType, analyticscontracts.OperationTypeDlqCreated, "failed", wsID, "non_retryable", c.name, "", now)
				}
				if c.markers != nil {
					if _, mErr := c.markers.MarkProcessed(ctx, c.name, eventID); mErr != nil {
						return mErr
					}
				}
				return nil
			}

			c.log.Error("retryable error handling provider event",
				"event_id", eventID,
				"error", err,
			)
			return err
		}

		if c.markers != nil {
			if _, err := c.markers.MarkProcessed(ctx, c.name, eventID); err != nil {
				c.log.Error("failed to mark event as processed, returning for retry",
					"event_id", eventID, "error", err,
				)
				return err
			}
		}
		return nil
	})
}

func (c *ProviderEventConsumer) HandleEvent(ctx context.Context, eventID string, rawPayload []byte) error {
	envelope, err := events.Unmarshal(rawPayload)
	if err != nil {
		return &deliveryapp.NonRetryableError{Err: err}
	}

	if envelope.EventType != ingestioncontracts.EventProviderEventNormalizedV1 {
		c.log.Warn("unexpected event type, ignoring",
			"event_id", envelope.EventID,
			"expected", ingestioncontracts.EventProviderEventNormalizedV1,
			"actual", envelope.EventType,
		)
		return nil
	}

	var payload ingestioncontracts.ProviderEventNormalizedPayload
	if err := envelope.DecodePayload(&payload); err != nil {
		return &deliveryapp.NonRetryableError{Err: err}
	}

	occurredAt, err := time.Parse(time.RFC3339, payload.OccurredAt)
	if err != nil {
		return &deliveryapp.NonRetryableError{Err: err}
	}

	receivedAt, err := time.Parse(time.RFC3339, payload.ReceivedAt)
	if err != nil {
		return &deliveryapp.NonRetryableError{Err: err}
	}

	input := deliveryapp.HandleProviderEventInput{
		EventID:           eventID,
		NormalizedEventID: payload.NormalizedEventID,
		RawEventID:        payload.RawEventID,
		WorkspaceID:       payload.WorkspaceID,
		MessageID:         payload.MessageID,
		Provider:          payload.Provider,
		ProviderEventID:   payload.ProviderEventID,
		ProviderMessageID: payload.ProviderMessageID,
		EventType:         payload.EventType,
		OccurredAt:        occurredAt,
		ReceivedAt:        receivedAt,
	}

	result, err := c.svc.HandleProviderEvent(ctx, input)
	if err != nil {
		return err
	}

	if result.Ignored {
		c.log.Info("provider event ignored",
			"event_id", envelope.EventID,
			"event_type", payload.EventType,
		)
	} else {
		c.log.Info("provider event handled successfully",
			"event_id", envelope.EventID,
			"normalized_event_id", payload.NormalizedEventID,
			"message_id", result.MessageID,
			"workspace_id", result.WorkspaceID,
			"previous_status", result.PreviousStatus,
			"new_status", result.NewStatus,
			"suppression_created", result.SuppressionCreated,
		)
	}

	return nil
}
