package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/webhooks/contracts"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/events"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/id"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/kafka"
)

const (
	defaultDueWebhookSchedulerInterval = 10 * time.Second
	defaultDueWebhookSchedulerMaxEmpty = 5
	defaultDueWebhookSchedulerMaxWait  = 10 * time.Second
)

// DueWebhookScheduler periodically publishes a due-webhook-deliveries event to Kafka.
// The consumer claims pending deliveries and processes them.
type DueWebhookScheduler struct {
	name      string
	producer  kafka.Producer
	log       *slog.Logger
	batchSize int
	idGen     func() (string, error)
	guard     *PollingGuard
}

func NewDueWebhookScheduler(
	producer kafka.Producer,
	log *slog.Logger,
	batchSize int,
) *DueWebhookScheduler {
	name := "webhooks.due_webhook_scheduler"
	return &DueWebhookScheduler{
		name:      name,
		producer:  producer,
		log:       log.With("worker", name),
		batchSize: batchSize,
		idGen:     id.NewUUIDGenerator().New,
		guard:     NewPollingGuard(name, defaultDueWebhookSchedulerInterval, defaultDueWebhookSchedulerMaxEmpty, defaultDueWebhookSchedulerMaxWait, log),
	}
}

func (s *DueWebhookScheduler) Name() string { return s.name }

func (s *DueWebhookScheduler) Run(ctx context.Context) error {
	return s.guard.Run(ctx, s)
}

func (s *DueWebhookScheduler) Poll(ctx context.Context) (bool, error) {
	now := time.Now().UTC()

	eventID, err := s.idGen()
	if err != nil {
		return false, err
	}

	envelope, err := events.NewEnvelope(events.NewEnvelopeOptions{
		EventID:       eventID,
		EventType:     contracts.EventDueDeliveriesProcessV1,
		EventVersion:  1,
		AggregateType: "webhook_delivery",
		AggregateID:   "due_deliveries",
		OccurredAt:    now,
	}, contracts.DueDeliveriesProcessPayload{
		Limit: s.batchSize,
		Now:   now.Format(time.RFC3339Nano),
	})
	if err != nil {
		return false, err
	}

	msg, err := kafka.NewMessageFromEnvelope(envelope)
	if err != nil {
		return false, err
	}

	if err := s.producer.Produce(ctx, msg); err != nil {
		s.log.Error("failed to produce due webhook event", "error", err)
		return false, err
	}

	s.log.Debug("published due webhook delivery event")
	return true, nil
}
