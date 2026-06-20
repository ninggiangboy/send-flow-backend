package worker

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/contracts"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/events"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/id"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/kafka"
)

const (
	defaultDueMsgSchedulerInterval = 5 * time.Second
	defaultDueMsgSchedulerMaxEmpty = 5
	defaultDueMsgSchedulerMaxWait  = 5 * time.Second
)

// DueMessageScheduler periodically discovers workspaces with due messages and
// publishes per-workspace events to Kafka for consumer-side processing.
type DueMessageScheduler struct {
	name           string
	listWorkspaces func(ctx context.Context, messageType string, now time.Time) ([]string, error)
	producer       kafka.Producer
	log            *slog.Logger
	messageType    string
	batchSize      int
	idGen          func() (string, error)
	guard          *PollingGuard
}

func NewDueMessageScheduler(
	listWorkspaces func(ctx context.Context, messageType string, now time.Time) ([]string, error),
	producer kafka.Producer,
	log *slog.Logger,
	messageType string,
	batchSize int,
) *DueMessageScheduler {
	name := dueMessageSchedulerName(messageType)
	return &DueMessageScheduler{
		name:           name,
		listWorkspaces: listWorkspaces,
		producer:       producer,
		log:            log.With("worker", name),
		messageType:    messageType,
		batchSize:      batchSize,
		idGen:          id.NewUUIDGenerator().New,
		guard:          NewPollingGuard(name, defaultDueMsgSchedulerInterval, defaultDueMsgSchedulerMaxEmpty, defaultDueMsgSchedulerMaxWait, log),
	}
}

func (s *DueMessageScheduler) Name() string { return s.name }

func (s *DueMessageScheduler) Run(ctx context.Context) error {
	return s.guard.Run(ctx, s)
}

func (s *DueMessageScheduler) Poll(ctx context.Context) (bool, error) {
	workspaces, err := s.listWorkspaces(ctx, s.messageType, time.Now().UTC())
	if err != nil {
		return false, err
	}

	if len(workspaces) == 0 {
		return false, nil
	}

	published := 0
	now := time.Now().UTC()
	for _, wsID := range workspaces {
		eventID, err := s.idGen()
		if err != nil {
			s.log.Error("failed to generate event ID", "error", err)
			continue
		}

		envelope, err := events.NewEnvelope(events.NewEnvelopeOptions{
			EventID:       eventID,
			EventType:     contracts.EventDueMessagesProcessV1,
			EventVersion:  1,
			AggregateType: contracts.AggregateWorkspace,
			AggregateID:   wsID,
			WorkspaceID:   wsID,
			OccurredAt:    now,
		}, contracts.DueMessagesProcessPayload{
			WorkspaceID: wsID,
			MessageType: s.messageType,
			Limit:       s.batchSize,
			Now:         now.Format(time.RFC3339Nano),
		})
		if err != nil {
			s.log.Error("failed to create envelope", "workspace_id", wsID, "error", err)
			continue
		}

		msg, err := kafka.NewMessageFromEnvelope(envelope)
		if err != nil {
			s.log.Error("failed to create kafka message", "workspace_id", wsID, "error", err)
			continue
		}

		if err := s.producer.Produce(ctx, msg); err != nil {
			s.log.Error("failed to produce due message event", "workspace_id", wsID, "error", err)
			continue
		}
		published++
	}

	if published > 0 {
		s.log.Info("published due message events",
			"workspaces_found", len(workspaces),
			"published", published,
		)
	}

	return published > 0, nil
}

func dueMessageSchedulerName(messageType string) string {
	switch strings.TrimSpace(messageType) {
	case "", "marketing":
		return "delivery.due_message_scheduler"
	case "transactional":
		return "delivery.due_message_scheduler_tx"
	default:
		return "delivery.due_message_scheduler_" + strings.ReplaceAll(strings.TrimSpace(messageType), " ", "_")
	}
}
