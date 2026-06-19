package worker

import (
	"context"
	"log/slog"
	"strings"
	"time"

	deliveryapp "github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/app"
)

type DueMessageProcessor struct {
	name         string
	svc          *deliveryapp.Service
	log          *slog.Logger
	pollInterval time.Duration
	batchSize    int
	messageType  string
	guard        *PollingGuard
}

func NewDueMessageProcessor(svc *deliveryapp.Service, log *slog.Logger, pollInterval time.Duration, batchSize int, messageType string) *DueMessageProcessor {
	return newDueMessageProcessor(dueMessageProcessorName(messageType), svc, log, pollInterval, batchSize, messageType)
}

func newDueMessageProcessor(name string, svc *deliveryapp.Service, log *slog.Logger, pollInterval time.Duration, batchSize int, messageType string) *DueMessageProcessor {
	return &DueMessageProcessor{
		name:         name,
		svc:          svc,
		log:          log.With("worker", name),
		pollInterval: pollInterval,
		batchSize:    batchSize,
		messageType:  messageType,
		guard:        NewPollingGuard(name, pollInterval, 0, pollInterval, log),
	}
}

func (p *DueMessageProcessor) Name() string {
	return p.name
}

func (p *DueMessageProcessor) RunnerKey() string {
	return dueMessageProcessorKey(p.messageType)
}

func (p *DueMessageProcessor) Run(ctx context.Context) error {
	return p.guard.Run(ctx, p)
}

func (p *DueMessageProcessor) Poll(ctx context.Context) (bool, error) {
	workspaces, err := p.svc.ProcessDueMessagesAllWorkspaces(ctx, deliveryapp.ProcessDueMessagesAllInput{
		MessageType: p.messageType,
		Limit:       p.batchSize,
		Now:         time.Now().UTC(),
	})
	if err != nil {
		p.log.Error("failed to process due messages", "error", err)
		return false, err
	}

	if workspaces > 0 {
		p.log.Debug("processed due messages across workspaces", "workspaces_processed", workspaces)
	}
	return workspaces > 0, nil
}

func dueMessageProcessorName(messageType string) string {
	switch strings.TrimSpace(messageType) {
	case "", "marketing":
		return "delivery.process_due_messages"
	case "transactional":
		return "delivery.process_due_messages_tx"
	default:
		return "delivery.process_due_messages_" + strings.ReplaceAll(strings.TrimSpace(messageType), " ", "_")
	}
}

func dueMessageProcessorKey(messageType string) string {
	switch strings.TrimSpace(messageType) {
	case "", "marketing":
		return "delivery.due_messages.db_processor"
	case "transactional":
		return "delivery.due_messages_tx.db_processor"
	default:
		return "delivery.due_messages." + strings.ReplaceAll(strings.TrimSpace(messageType), " ", "_") + ".db_processor"
	}
}
