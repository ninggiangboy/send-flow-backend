package worker

import (
	"context"
	"log/slog"
	"time"

	webhooksapp "github.com/ninggiangboy/send-flow/backend/internal/modules/webhooks/app"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/webhooks/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/batching"
)

type dueWebhookDeliveryOutcome struct {
	delivery domain.WebhookDelivery
	outcome  string
}

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

	batchLimit := p.batchSize
	if batchLimit <= 0 {
		batchLimit = len(deliveries)
		if batchLimit <= 0 {
			batchLimit = 1
		}
	}
	idx := 0
	pipeline, err := batching.NewPipeline[domain.WebhookDelivery, dueWebhookDeliveryOutcome](batching.Config[domain.WebhookDelivery, dueWebhookDeliveryOutcome]{
		Options: batching.Options{
			BufferedItemsSize:    batchLimit,
			WriteBatchSize:       batchLimit,
			ProcessorConcurrency: 1,
			MaxInflight:          batchLimit,
		},
		Reader: batching.ItemReaderFunc[domain.WebhookDelivery](func(ctx context.Context, _ int, _ int) (domain.WebhookDelivery, bool, error) {
			if idx >= len(deliveries) {
				return domain.WebhookDelivery{}, false, nil
			}
			delivery := deliveries[idx]
			idx++
			return delivery, true, nil
		}),
		Processor: batching.ItemProcessorFunc[domain.WebhookDelivery, dueWebhookDeliveryOutcome](func(ctx context.Context, delivery domain.WebhookDelivery) (dueWebhookDeliveryOutcome, bool, error) {
			outcome, err := p.svc.ProcessDueDelivery(ctx, delivery.WorkspaceID, delivery.ID)
			if err != nil {
				p.log.Error("failed to process due delivery",
					"delivery_id", delivery.ID,
					"error", err,
				)
				return dueWebhookDeliveryOutcome{}, false, nil
			}
			return dueWebhookDeliveryOutcome{delivery: delivery, outcome: outcome}, true, nil
		}),
		Writer: batching.ItemWriterFunc[dueWebhookDeliveryOutcome](func(ctx context.Context, outcomes []dueWebhookDeliveryOutcome) error {
			for _, item := range outcomes {
				p.recordDeliveryOperation(ctx, item.delivery, item.outcome)
			}
			return nil
		}),
	})
	if err != nil {
		return false, err
	}
	if err := pipeline.Run(ctx); err != nil {
		return false, err
	}
	return len(deliveries) > 0, nil
}

func (p *DueWebhookDeliveryProcessor) recordDeliveryOperation(ctx context.Context, delivery domain.WebhookDelivery, outcome string) {
	if p.recordOp == nil || delivery.WorkspaceID == "" {
		return
	}

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
