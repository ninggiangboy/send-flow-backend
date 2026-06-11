package worker

import (
	"context"
	"log/slog"
	"time"

	notificationapp "github.com/ninggiangboy/send-flow/backend/internal/modules/notification/app"
)

type DueNotificationProcessor struct {
	name         string
	svc          *notificationapp.Service
	log          *slog.Logger
	pollInterval time.Duration
	batchSize    int
}

func NewDueNotificationProcessor(svc *notificationapp.Service, log *slog.Logger, pollInterval time.Duration, batchSize int) *DueNotificationProcessor {
	return &DueNotificationProcessor{
		name:         "notification.process_due_retries",
		svc:          svc,
		log:          log.With("worker", "notification.process_due_retries"),
		pollInterval: pollInterval,
		batchSize:    batchSize,
	}
}

func (p *DueNotificationProcessor) Name() string {
	return p.name
}

func (p *DueNotificationProcessor) Run(ctx context.Context) error {
	p.log.Info("starting due notification retry processor",
		"poll_interval", p.pollInterval,
		"batch_size", p.batchSize,
	)

	ticker := time.NewTicker(p.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			p.log.Info("due notification retry processor stopped")
			return nil
		case <-ticker.C:
			p.processOnce(ctx)
		}
	}
}

func (p *DueNotificationProcessor) processOnce(ctx context.Context) {
	processed, err := p.svc.ProcessRetryBatch(ctx, p.batchSize)
	if err != nil {
		p.log.Error("failed to process notification retry batch", "error", err)
		return
	}

	if processed > 0 {
		p.log.Debug("processed notification retries", "count", processed)
	}
}
