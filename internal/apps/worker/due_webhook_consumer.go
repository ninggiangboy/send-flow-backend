package worker

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	webhooksapp "github.com/ninggiangboy/send-flow/backend/internal/modules/webhooks/app"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/webhooks/contracts"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/events"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/id"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/kafka"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/retryable"
)

// DueWebhookConsumer processes due webhook delivery events from Kafka.
type DueWebhookConsumer struct {
	name       string
	svc        *webhooksapp.Service
	log        *slog.Logger
	brokers    []string
	groupID    string
	markers    *ProcessedEventMarkers
	deadLetter *DeadLetterRepository
	recordOp   OperationsEventRecorder
	idGen      func() (string, error)
}

func NewDueWebhookConsumer(svc *webhooksapp.Service, log *slog.Logger, brokers []string, groupID string, pool *pgxpool.Pool) *DueWebhookConsumer {
	c := &DueWebhookConsumer{
		name:    "webhooks.due_webhook_consumer",
		svc:     svc,
		log:     log.With(logFieldConsumer, consumerWebhooksDue),
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

func (c *DueWebhookConsumer) SetOperationsRecorder(r OperationsEventRecorder) {
	c.recordOp = r
}

func (c *DueWebhookConsumer) Name() string { return c.name }

func (c *DueWebhookConsumer) Run(ctx context.Context) error {
	if len(c.brokers) == 0 {
		c.log.Info("kafka not configured, consumer disabled")
		<-ctx.Done()
		return nil
	}

	topic := events.TopicFromEventType(contracts.EventDueDeliveriesProcessV1)

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
			var nonRetryable *retryable.NonRetryableError
			if errors.As(err, &nonRetryable) {
				c.log.Warn("non-retryable error handling due webhook event",
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

			c.log.Error("retryable error handling due webhook event",
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

func (c *DueWebhookConsumer) HandleEvent(ctx context.Context, eventID string, rawPayload []byte) error {
	env, err := events.Unmarshal(rawPayload)
	if err != nil {
		c.log.Warn("failed to unmarshal due webhook envelope", "event_id", eventID, "error", err)
		return &retryable.NonRetryableError{Err: err}
	}

	var payload contracts.DueDeliveriesProcessPayload
	if err := json.Unmarshal(env.Payload, &payload); err != nil {
		c.log.Warn("failed to unmarshal due webhook payload", "event_id", eventID, "error", err)
		return &retryable.NonRetryableError{Err: err}
	}

	now, err := time.Parse(time.RFC3339Nano, payload.Now)
	if err != nil {
		now = time.Now().UTC()
	}

	deliveries, err := c.svc.ClaimDueDeliveries(ctx, payload.Limit, now)
	if err != nil {
		return err
	}

	for _, delivery := range deliveries {
		outcome, err := c.svc.ProcessDueDelivery(ctx, delivery.WorkspaceID, delivery.ID)
		if err != nil {
			c.log.Error("failed to process due delivery",
				"delivery_id", delivery.ID,
				"error", err,
			)
			continue
		}
		if c.recordOp != nil && delivery.WorkspaceID != "" {
			var status string
			switch outcome {
			case "succeeded":
				status = "success"
			case "failed":
				status = "failure"
			case "retry_scheduled":
				status = "retry"
			default:
				status = outcome
			}
			opType := "webhook_" + outcome
			c.recordOp(ctx, "webhooks", "webhook.delivery", opType, status, delivery.WorkspaceID, "", "webhooks.due_webhook_consumer", delivery.TargetURL, time.Now())
		}
	}

	c.log.Info("due webhook event handled",
		"event_id", eventID,
		"deliveries_processed", len(deliveries),
	)
	return nil
}
