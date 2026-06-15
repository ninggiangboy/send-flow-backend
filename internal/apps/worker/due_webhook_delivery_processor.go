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
	recordOp     OperationsEventRecorder
	guard        *PollingGuard
}

func NewDueWebhookDeliveryProcessor(svc *webhooksapp.Service, log *slog.Logger, pollInterval time.Duration, batchSize int) *DueWebhookDeliveryProcessor {
	return newDueWebhookDeliveryProcessor("webhooks.process_due_deliveries", svc, log, pollInterval, batchSize)
}

func newDueWebhookDeliveryProcessor(name string, svc *webhooksapp.Service, log *slog.Logger, pollInterval time.Duration, batchSize int) *DueWebhookDeliveryProcessor {
	return &DueWebhookDeliveryProcessor{
		name:         name,
		svc:          svc,
		log:          log.With("worker", name),
		pollInterval: pollInterval,
		batchSize:    batchSize,
		guard:        NewPollingGuard(name, pollInterval, 0, pollInterval, log),
	}
}

func (p *DueWebhookDeliveryProcessor) SetOperationsRecorder(r OperationsEventRecorder) {
	p.recordOp = r
}

func (p *DueWebhookDeliveryProcessor) Name() string {
	return p.name
}

func (p *DueWebhookDeliveryProcessor) RunnerKey() string {
	return "webhooks.due_deliveries.db_processor"
}

func (p *DueWebhookDeliveryProcessor) Run(ctx context.Context) error {
	return p.guard.Run(ctx, p)
}

func (p *DueWebhookDeliveryProcessor) Poll(ctx context.Context) (bool, error) {
	now := time.Now().UTC()
	deliveries, err := p.svc.ClaimDueDeliveries(ctx, p.batchSize, now)
	if err != nil {
		p.log.Error("failed to claim due deliveries", "error", err)
		return false, err
	}

	for _, delivery := range deliveries {
		outcome, err := p.svc.ProcessDueDelivery(ctx, delivery.WorkspaceID, delivery.ID)
		if err != nil {
			p.log.Error("failed to process due delivery",
				"delivery_id", delivery.ID,
				"error", err,
			)
			continue
		}
		if p.recordOp != nil && delivery.WorkspaceID != "" {
			var status string
			switch outcome {
			case "succeeded":
				status = "success"
			case "failed":
				status = "failure"
			case "retry_scheduled":
				status = "retry"
			default:
				status = outcome
			}
			opType := "webhook_" + outcome
			p.recordOp(ctx, "webhooks", "webhook.delivery", opType, status, delivery.WorkspaceID, "", "webhooks.process_due_deliveries", delivery.TargetURL, time.Now())
		}
	}
	return len(deliveries) > 0, nil
}
