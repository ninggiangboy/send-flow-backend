package worker

import (
	"context"
	"log/slog"
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
}

func NewDueMessageProcessor(svc *deliveryapp.Service, log *slog.Logger, pollInterval time.Duration, batchSize int, messageType string) *DueMessageProcessor {
	return &DueMessageProcessor{
		name:         "delivery.process_due_messages",
		svc:          svc,
		log:          log.With("worker", "delivery.process_due_messages"),
		pollInterval: pollInterval,
		batchSize:    batchSize,
		messageType:  messageType,
	}
}

func (p *DueMessageProcessor) Name() string {
	return p.name
}

func (p *DueMessageProcessor) Run(ctx context.Context) error {
	p.log.Info("starting due message processor",
		"poll_interval", p.pollInterval,
		"batch_size", p.batchSize,
		"message_type", p.messageType,
	)

	ticker := time.NewTicker(p.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			p.log.Info("due message processor stopped")
			return nil
		case <-ticker.C:
			p.processOnce(ctx)
		}
	}
}

func (p *DueMessageProcessor) processOnce(ctx context.Context) {
	workspaces, err := p.svc.ProcessDueMessagesAllWorkspaces(ctx, deliveryapp.ProcessDueMessagesAllInput{
		MessageType: p.messageType,
		Limit:       p.batchSize,
		Now:         time.Now().UTC(),
	})
	if err != nil {
		p.log.Error("failed to process due messages", "error", err)
		return
	}

	if workspaces > 0 {
		p.log.Debug("processed due messages across workspaces", "workspaces_processed", workspaces)
	}
}
