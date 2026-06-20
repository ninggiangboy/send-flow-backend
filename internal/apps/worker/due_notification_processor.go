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
	guard        *PollingGuard
}

func NewDueNotificationProcessor(svc *notificationapp.Service, log *slog.Logger, pollInterval time.Duration, batchSize int) *DueNotificationProcessor {
	return newDueNotificationProcessor("notification.process_due_retries", svc, log, pollInterval, batchSize)
}

func newDueNotificationProcessor(name string, svc *notificationapp.Service, log *slog.Logger, pollInterval time.Duration, batchSize int) *DueNotificationProcessor {
	return &DueNotificationProcessor{
		name:         name,
		svc:          svc,
		log:          log.With(logFieldWorker, name),
		pollInterval: pollInterval,
		batchSize:    batchSize,
		guard:        NewPollingGuard(name, pollInterval, 0, pollInterval, log),
	}
}

func (p *DueNotificationProcessor) Name() string {
	return p.name
}

func (p *DueNotificationProcessor) RunnerKey() string {
	return "notification.due_retries.db_processor"
}

func (p *DueNotificationProcessor) Run(ctx context.Context) error {
	return p.guard.Run(ctx, p)
}

func (p *DueNotificationProcessor) Poll(ctx context.Context) (bool, error) {
	processed, err := p.svc.ProcessRetryBatch(ctx, p.batchSize)
	if err != nil {
		p.log.Error("failed to process notification retry batch", "error", err)
		return false, err
	}

	if processed > 0 {
		p.log.Debug("processed notification retries", "count", processed)
	}
	return processed > 0, nil
}
