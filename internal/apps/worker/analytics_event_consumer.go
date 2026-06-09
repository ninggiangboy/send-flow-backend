package worker

import (
	"context"
	"errors"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	analyticsapp "github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/app"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/events"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/id"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/kafka"
)

type AnalyticsEventConsumer struct {
	name       string
	svc        *analyticsapp.Service
	log        *slog.Logger
	brokers    []string
	groupID    string
	markers    *ProcessedEventMarkers
	deadLetter *DeadLetterRepository
	idGen      func() (string, error)
}

func NewAnalyticsEventConsumer(svc *analyticsapp.Service, log *slog.Logger, brokers []string, groupID string, pool *pgxpool.Pool) *AnalyticsEventConsumer {
	c := &AnalyticsEventConsumer{
		name:    "analytics_events",
		svc:     svc,
		log:     log.With("consumer", "analytics_events"),
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

func (c *AnalyticsEventConsumer) Name() string {
	return c.name
}

func (c *AnalyticsEventConsumer) Run(ctx context.Context) error {
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

	c.log.Info("starting analytics kafka consumer", "topics", topics, "group_id", c.groupID)

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
			var nonRetryable *NonRetryableError
			if errors.As(err, &nonRetryable) {
				c.log.Warn("non-retryable error handling analytics event",
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

			c.log.Error("retryable error handling analytics event",
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

func (c *AnalyticsEventConsumer) HandleEvent(ctx context.Context, eventID string, rawPayload []byte) error {
	envelope, err := events.Unmarshal(rawPayload)
	if err != nil {
		return &NonRetryableError{Err: err}
	}

	mapped, err := analyticsapp.MapEnvelopeToEvent(envelope)
	if err != nil {
		if errors.Is(err, analyticsapp.ErrUnsupportedEventType) {
			c.log.Info("unsupported event type, ignoring",
				"event_id", envelope.EventID,
				"event_type", envelope.EventType,
			)
			return nil
		}
		return &NonRetryableError{Err: err}
	}

	if mapped.Input.WorkspaceID == "" {
		c.log.Warn("event missing workspace_id, skipping",
			"event_id", envelope.EventID,
			"event_type", envelope.EventType,
		)
		return nil
	}

	c.log.Info("processing analytics event",
		"event_id", envelope.EventID,
		"event_type", envelope.EventType,
		"workspace_id", mapped.Input.WorkspaceID,
		"campaign_id", mapped.Input.CampaignID,
		"message_id", mapped.Input.MessageID,
	)

	if err := c.svc.IngestEmailEventFact(ctx, mapped.Input); err != nil {
		if errors.Is(err, domain.ErrAnalyticsEventInvalid) {
			return &NonRetryableError{Err: err}
		}
		return err
	}

	return nil
}

type NonRetryableError struct {
	Err error
}

func (e *NonRetryableError) Error() string {
	return "non-retryable: " + e.Err.Error()
}

func (e *NonRetryableError) Unwrap() error {
	return e.Err
}
