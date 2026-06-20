package worker

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	notificationapp "github.com/ninggiangboy/send-flow/backend/internal/modules/notification/app"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/notification/contracts"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/events"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/id"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/kafka"
)

// DueNotificationConsumer processes due notification retry events from Kafka.
type DueNotificationConsumer struct {
	name       string
	svc        *notificationapp.Service
	log        *slog.Logger
	brokers    []string
	groupID    string
	markers    *ProcessedEventMarkers
	deadLetter *DeadLetterRepository
	idGen      func() (string, error)
}

func NewDueNotificationConsumer(svc *notificationapp.Service, log *slog.Logger, brokers []string, groupID string, pool *pgxpool.Pool) *DueNotificationConsumer {
	c := &DueNotificationConsumer{
		name:    "notification.due_notification_consumer",
		svc:     svc,
		log:     log.With(logFieldConsumer, consumerNotificationDue),
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

func (c *DueNotificationConsumer) Name() string { return c.name }

func (c *DueNotificationConsumer) Run(ctx context.Context) error {
	if len(c.brokers) == 0 {
		c.log.Info("kafka not configured, consumer disabled")
		<-ctx.Done()
		return nil
	}

	topic := events.TopicFromEventType(contracts.EventDueNotificationsProcessV1)

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
			var nonRetryable *notificationapp.NonRetryableError
			if errors.As(err, &nonRetryable) {
				c.log.Warn("non-retryable error handling due notification event",
					"event_id", eventID,
					"error", err,
				)
				if c.deadLetter != nil {
					if dlErr := c.deadLetter.Save(ctx, DeadLetterRecord{
						ID:              mustNewID(c.idGen),
						WorkspaceID:     workspaceIDFromMessage(msg.Headers, msg.Value),
						Source:          c.name,
						SourceEventType: eventTypeFromEnvelope(msg.Value),
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
				if c.markers != nil {
					if _, mErr := c.markers.MarkProcessed(ctx, c.name, eventID); mErr != nil {
						return mErr
					}
				}
				return nil
			}

			c.log.Error("retryable error handling due notification event",
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

func (c *DueNotificationConsumer) HandleEvent(ctx context.Context, eventID string, rawPayload []byte) error {
	env, err := events.Unmarshal(rawPayload)
	if err != nil {
		c.log.Warn("failed to unmarshal due notification envelope", "event_id", eventID, "error", err)
		return &notificationapp.NonRetryableError{Err: err}
	}

	var payload contracts.DueNotificationsProcessPayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		c.log.Warn("failed to unmarshal due notification payload", "event_id", eventID, "error", err)
		return &notificationapp.NonRetryableError{Err: err}
	}

	processed, err := c.svc.ProcessRetryBatch(ctx, payload.BatchSize)
	if err != nil {
		return err
	}

	c.log.Info("due notification event handled",
		"event_id", eventID,
		"retries_processed", processed,
	)
	return nil
}
