package worker

import (
	"context"
	"log/slog"
	"time"

	webhooksapp "github.com/ninggiangboy/send-flow/backend/internal/modules/webhooks/app"
)

type DueWebhookDeliveryProcessor struct {
	name         string
	svc          *webhooksapp.Service
	log          *slog.Logger
	pollInterval time.Duration
	batchSize    int
}

func NewDueWebhookDeliveryProcessor(svc *webhooksapp.Service, log *slog.Logger, pollInterval time.Duration, batchSize int) *DueWebhookDeliveryProcessor {
	return &DueWebhookDeliveryProcessor{
		name:         "webhooks.process_due_deliveries",
		svc:          svc,
		log:          log.With("worker", "webhooks.process_due_deliveries"),
		pollInterval: pollInterval,
		batchSize:    batchSize,
	}
}

func (p *DueWebhookDeliveryProcessor) Name() string {
	return p.name
}

func (p *DueWebhookDeliveryProcessor) Run(ctx context.Context) error {
	p.log.Info("starting due webhook delivery processor",
		"poll_interval", p.pollInterval,
		"batch_size", p.batchSize,
	)

	ticker := time.NewTicker(p.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			p.log.Info("due webhook delivery processor stopped")
			return nil
		case <-ticker.C:
			p.processOnce(ctx)
		}
	}
}

func (p *DueWebhookDeliveryProcessor) processOnce(ctx context.Context) {
	now := time.Now().UTC()
	deliveries, err := p.svc.ClaimDueDeliveries(ctx, p.batchSize, now)
	if err != nil {
		p.log.Error("failed to claim due deliveries", "error", err)
		return
	}

	for _, delivery := range deliveries {
		if err := p.svc.ProcessDueDelivery(ctx, delivery.WorkspaceID, delivery.ID); err != nil {
			p.log.Error("failed to process due delivery",
				"delivery_id", delivery.ID,
				"error", err,
			)
		}
	}
}
