package worker

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	analyticsapp "github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/app"
	analyticscontracts "github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/contracts"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/events"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/id"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/kafka"
	platformerrors "github.com/ninggiangboy/send-flow/backend/internal/platform/retryable"
)

type AnalyticsEventConsumer struct {
	name       string
	svc        *analyticsapp.Service
	registry   *analyticsapp.MapperRegistry
	log        *slog.Logger
	brokers    []string
	groupID    string
	markers    *ProcessedEventMarkers
	deadLetter *DeadLetterRepository
	idGen      func() (string, error)
}

func NewAnalyticsEventConsumer(svc *analyticsapp.Service, registry *analyticsapp.MapperRegistry, log *slog.Logger, brokers []string, groupID string, pool *pgxpool.Pool) *AnalyticsEventConsumer {
	c := &AnalyticsEventConsumer{
		name:     "analytics_events",
		svc:      svc,
		registry: registry,
		log:      log.With("consumer", "analytics_events"),
		brokers:  brokers,
		groupID:  groupID,
		idGen:    id.NewUUIDGenerator().New,
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

	eventTypes := c.registry.EventTypes()
	topics := make([]string, len(eventTypes))
	for i, et := range eventTypes {
		topics[i] = events.TopicFromEventType(et)
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
			var nonRetryable *platformerrors.NonRetryableError
			if errors.As(err, &nonRetryable) {
				c.log.Warn("non-retryable error handling analytics event",
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
				if wsID != "" {
					if opErr := c.svc.IngestOperationsEvent(ctx, analyticsapp.IngestOperationsEventInput{
						Source:          c.name,
						SourceEventType: evType,
						OperationType:   analyticscontracts.OperationTypeConsumerFailure,
						Status:          analyticscontracts.OperationStatusFailure,
						WorkspaceID:     wsID,
						ErrorType:       "non_retryable",
						Consumer:        c.name,
						OccurredAt:      time.Now(),
					}); opErr != nil {
						c.log.Error("failed to record consumer failure in clickhouse",
							"event_id", eventID, "error", opErr,
						)
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
		return &platformerrors.NonRetryableError{Err: err}
	}

	mapped, err := c.registry.MapEvent(envelope)
	if err != nil {
		if errors.Is(err, analyticsapp.ErrUnsupportedEventType) {
			c.log.Info("unsupported event type, ignoring",
				"event_id", envelope.EventID,
				"event_type", envelope.EventType,
			)
			return nil
		}
		return &platformerrors.NonRetryableError{Err: err}
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
			return &platformerrors.NonRetryableError{Err: err}
		}
		return err
	}

	return nil
}
