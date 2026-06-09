package worker

import (
	"context"
	"errors"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	webhooksapp "github.com/ninggiangboy/send-flow/backend/internal/modules/webhooks/app"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/events"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/id"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/kafka"
)

type WebhookEventConsumer struct {
	name       string
	svc        *webhooksapp.Service
	log        *slog.Logger
	brokers    []string
	groupID    string
	markers    *ProcessedEventMarkers
	deadLetter *DeadLetterRepository
	idGen      func() (string, error)
}

func NewWebhookEventConsumer(svc *webhooksapp.Service, log *slog.Logger, brokers []string, groupID string, pool *pgxpool.Pool) *WebhookEventConsumer {
	c := &WebhookEventConsumer{
		name:    "webhooks.deliver_events",
		svc:     svc,
		log:     log.With("consumer", "webhooks.deliver_events"),
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

func (c *WebhookEventConsumer) Name() string {
	return c.name
}

func (c *WebhookEventConsumer) Run(ctx context.Context) error {
	if len(c.brokers) == 0 {
		c.log.Info("kafka not configured, consumer disabled")
		<-ctx.Done()
		return nil
	}

	topics := []string{
		events.TopicFromEventType("delivery.message.queued.v1"),
		events.TopicFromEventType("delivery.message.accepted.v1"),
		events.TopicFromEventType("delivery.message.delivered.v1"),
		events.TopicFromEventType("delivery.message.bounced.v1"),
		events.TopicFromEventType("delivery.message.complained.v1"),
		events.TopicFromEventType("delivery.message.retry_scheduled.v1"),
		events.TopicFromEventType("tracking.email_opened.v1"),
		events.TopicFromEventType("tracking.link_clicked.v1"),
		events.TopicFromEventType("tracking.recipient_unsubscribed.v1"),
		events.TopicFromEventType("suppression.recipient_suppressed.v1"),
	}

	consumer, err := kafka.NewReaderConsumer(kafka.ReaderConsumerOptions{
		Brokers:     c.brokers,
		GroupTopics: topics,
		GroupID:     c.groupID,
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

	c.log.Info("starting webhook event kafka consumer", "topics", topics, "group_id", c.groupID)

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
			var nonRetryable *webhooksapp.NonRetryableError
			if errors.As(err, &nonRetryable) {
				c.log.Warn("non-retryable error handling webhook event",
					"event_id", eventID,
					"error", err,
				)
				if c.deadLetter != nil {
					if dlErr := c.deadLetter.Save(ctx, DeadLetterRecord{
						ID:           mustNewID(c.idGen),
						Source:       c.name,
						EventID:      eventID,
						Payload:      msg.Value,
						ErrorMessage: err.Error(),
						Retryable:    false,
					}); dlErr != nil {
						c.log.Error("failed to save dead letter record, returning for retry",
							"event_id", eventID, "error", dlErr,
						)
						return dlErr
					}
				}
				if c.markers != nil {
					if _, mErr := c.markers.MarkProcessed(ctx, c.name, eventID); mErr != nil {
						return mErr
					}
				}
				return nil
			}

			c.log.Error("retryable error handling webhook event",
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

func (c *WebhookEventConsumer) HandleEvent(ctx context.Context, eventID string, rawPayload []byte) error {
	envelope, err := events.Unmarshal(rawPayload)
	if err != nil {
		return &webhooksapp.NonRetryableError{Err: err}
	}

	mapped, err := webhooksapp.MapEnvelopeToSourceEvent(envelope)
	if err != nil {
		if errors.Is(err, webhooksapp.ErrUnsupportedEventType) {
			c.log.Info("unsupported event type, ignoring",
				"event_id", envelope.EventID,
				"event_type", envelope.EventType,
			)
			return nil
		}
		return &webhooksapp.NonRetryableError{Err: err}
	}

	if mapped.WorkspaceID == "" {
		c.log.Warn("event missing workspace_id, skipping",
			"event_id", envelope.EventID,
			"event_type", envelope.EventType,
		)
		return nil
	}

	c.log.Info("processing webhook event",
		"event_id", envelope.EventID,
		"event_type", envelope.EventType,
		"workspace_id", mapped.WorkspaceID,
	)

	if err := c.svc.HandleSourceEvent(ctx, webhooksapp.HandleSourceEventInput{
		RawPayload: rawPayload,
	}); err != nil {
		var nonRetryable *webhooksapp.NonRetryableError
		if errors.As(err, &nonRetryable) {
			return nonRetryable
		}
		return err
	}

	return nil
}
