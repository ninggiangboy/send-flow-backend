package worker

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	deliveryapp "github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/app"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/contracts"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/events"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/id"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/kafka"
)

type CampaignScheduledConsumer struct {
	name       string
	svc        *deliveryapp.Service
	log        *slog.Logger
	brokers    []string
	groupID    string
	markers    *ProcessedEventMarkers
	deadLetter *DeadLetterRepository
	idGen      func() (string, error)
}

func NewCampaignScheduledConsumer(svc *deliveryapp.Service, log *slog.Logger, brokers []string, groupID string, pool *pgxpool.Pool) *CampaignScheduledConsumer {
	c := &CampaignScheduledConsumer{
		name:    "delivery.queue_campaign_messages",
		svc:     svc,
		log:     log.With(logFieldConsumer, consumerDeliveryQueueCampaign),
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

func (c *CampaignScheduledConsumer) Name() string {
	return c.name
}

func (c *CampaignScheduledConsumer) Run(ctx context.Context) error {
	if len(c.brokers) == 0 {
		c.log.Info("kafka not configured, consumer disabled")
		<-ctx.Done()
		return nil
	}

	topic := events.TopicFromEventType(contracts.EventCampaignScheduledV1)

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
				c.log.Warn("non-retryable error handling campaign scheduled event",
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
				// Dead letter saved — mark processed so we don't retry a permanent failure.
				if c.markers != nil {
					if _, mErr := c.markers.MarkProcessed(ctx, c.name, eventID); mErr != nil {
						return mErr
					}
				}
				return nil
			}

			c.log.Error("retryable error handling campaign scheduled event",
				"event_id", eventID,
				"error", err,
			)
			return err
		}

		// Only mark processed after the event is handled successfully.
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

func (c *CampaignScheduledConsumer) HandleEvent(ctx context.Context, eventID string, rawPayload []byte) error {
	env, err := events.Unmarshal(rawPayload)
	if err != nil {
		c.log.Warn("failed to unmarshal campaign scheduled envelope", "event_id", eventID, "error", err)
		return &deliveryapp.NonRetryableError{Err: err}
	}
	err = c.svc.HandleCampaignScheduled(ctx, deliveryapp.HandleCampaignScheduledInput{
		EventID:   eventID,
		EventType: env.EventType,
		Payload:   env.Payload,
		Now:       time.Now().UTC(),
	})
	if err != nil {
		var nonRetryable *deliveryapp.NonRetryableError
		if errors.As(err, &nonRetryable) {
			c.log.Warn("non-retryable error handling campaign scheduled event",
				"event_id", eventID,
				"error", err,
			)
		} else {
			c.log.Error("retryable error handling campaign scheduled event",
				"event_id", eventID,
				"error", err,
			)
		}
		return err
	}
	c.log.Info("campaign scheduled event handled successfully",
		"event_id", eventID,
	)
	return nil
}

func mustNewID(gen func() (string, error)) string {
	id, err := gen()
	if err != nil {
		panic(err)
	}
	return id
}
