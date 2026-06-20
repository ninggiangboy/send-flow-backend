package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/notification/contracts"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/events"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/id"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/kafka"
)

const (
	defaultDueNotifSchedulerInterval = 10 * time.Second
	defaultDueNotifSchedulerMaxEmpty = 5
	defaultDueNotifSchedulerMaxWait  = 10 * time.Second
)

// DueNotificationScheduler periodically publishes a due-notifications event to Kafka.
// The consumer calls ProcessRetryBatch to retry failed notification deliveries.
type DueNotificationScheduler struct {
	name      string
	producer  kafka.Producer
	log       *slog.Logger
	batchSize int
	idGen     func() (string, error)
	guard     *PollingGuard
}

func NewDueNotificationScheduler(
	producer kafka.Producer,
	log *slog.Logger,
	batchSize int,
) *DueNotificationScheduler {
	name := "notification.due_notification_scheduler"
	return &DueNotificationScheduler{
		name:      name,
		producer:  producer,
		log:       log.With(logFieldWorker, name),
		batchSize: batchSize,
		idGen:     id.NewUUIDGenerator().New,
		guard:     NewPollingGuard(name, defaultDueNotifSchedulerInterval, defaultDueNotifSchedulerMaxEmpty, defaultDueNotifSchedulerMaxWait, log),
	}
}

func (s *DueNotificationScheduler) Name() string { return s.name }

func (s *DueNotificationScheduler) Run(ctx context.Context) error {
	return s.guard.Run(ctx, s)
}

func (s *DueNotificationScheduler) Poll(ctx context.Context) (bool, error) {
	now := time.Now().UTC()

	eventID, err := s.idGen()
	if err != nil {
		return false, err
	}

	envelope, err := events.NewEnvelope(events.NewEnvelopeOptions{
		EventID:       eventID,
		EventType:     contracts.EventDueNotificationsProcessV1,
		EventVersion:  1,
		AggregateType: contracts.AggregateNotification,
		AggregateID:   "due_retries",
		OccurredAt:    now,
	}, contracts.DueNotificationsProcessPayload{
		BatchSize: s.batchSize,
		Now:       now.Format(time.RFC3339Nano),
	})
	if err != nil {
		return false, err
	}

	msg, err := kafka.NewMessageFromEnvelope(envelope)
	if err != nil {
		return false, err
	}

	if err := s.producer.Produce(ctx, msg); err != nil {
		s.log.Error("failed to produce due notification event", "error", err)
		return false, err
	}

	s.log.Debug("published due notification event")
	return true, nil
}
