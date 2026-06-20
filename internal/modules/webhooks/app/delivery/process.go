package delivery

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	webhookscontracts "github.com/ninggiangboy/send-flow/backend/internal/modules/webhooks/contracts"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/webhooks/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/webhooks/ports"
)

const maxWebhookRetries = 5

type ProcessOptions struct {
	DeliveryWrite   ports.DeliveryWriteRepository
	AttemptWrite    ports.AttemptWriteRepository
	ConfigWrite     ports.ConfigWriteRepository
	TxManager       ports.TransactionManager
	OutboxWriter    ports.OutboxWriter
	DeliverWebhookH *DeliverHandler
	IDGen           func() (string, error)
	Clock           func() time.Time
	Logger          *slog.Logger
}

type ProcessCommand struct {
	WorkspaceID string
	DeliveryID  string
}

type ProcessHandler struct {
	deliveryWrite   ports.DeliveryWriteRepository
	attemptWrite    ports.AttemptWriteRepository
	configWrite     ports.ConfigWriteRepository
	txManager       ports.TransactionManager
	outboxWriter    ports.OutboxWriter
	deliverWebhookH *DeliverHandler
	idGen           func() (string, error)
	clock           func() time.Time
	log             *slog.Logger
}

func NewProcess(opts ProcessOptions) *ProcessHandler {
	return &ProcessHandler{
		deliveryWrite:   opts.DeliveryWrite,
		attemptWrite:    opts.AttemptWrite,
		configWrite:     opts.ConfigWrite,
		txManager:       opts.TxManager,
		outboxWriter:    opts.OutboxWriter,
		deliverWebhookH: opts.DeliverWebhookH,
		idGen:           opts.IDGen,
		clock:           opts.Clock,
		log:             opts.Logger.With("usecase", "process_due_delivery"),
	}
}

func (h *ProcessHandler) Execute(ctx context.Context, cmd ProcessCommand) (string, error) {
	delivery, err := h.deliveryWrite.FindByID(ctx, cmd.WorkspaceID, cmd.DeliveryID)
	if err != nil {
		return "", fmt.Errorf("find delivery: %w", err)
	}

	cfg, err := h.configWrite.FindByID(ctx, delivery.WorkspaceID, delivery.WebhookID)
	if err != nil {
		return "", fmt.Errorf("find webhook config: %w", err)
	}

	if cfg.Status != domain.ConfigStatusActive {
		if err := h.deliveryWrite.MarkFailed(ctx, cmd.DeliveryID, domain.DeliveryResult{
			Error: "webhook config disabled",
		}); err != nil {
			return "", fmt.Errorf("mark failed: %w", err)
		}
		return "failed", nil
	}

	attemptNumber := int(delivery.AttemptCount)

	result, err := h.deliverWebhookH.Execute(ctx, DeliverCommand{
		WorkspaceID:     delivery.WorkspaceID,
		WebhookID:       cfg.ID,
		DeliveryID:      cmd.DeliveryID,
		TargetURL:       cfg.TargetURL,
		SecretHash:      cfg.SecretHash,
		EventPayload:    delivery.EventPayloadJSON,
		SourceEventID:   delivery.SourceEventID,
		SourceEventType: delivery.SourceEventType,
		AttemptNumber:   int64(attemptNumber),
	})
	if err != nil {
		return "", fmt.Errorf("deliver webhook: %w", err)
	}

	return h.recordDeliveryOutcome(ctx, delivery, cfg, attemptNumber, result)
}

func (h *ProcessHandler) recordDeliveryOutcome(ctx context.Context, delivery *domain.WebhookDelivery, cfg *domain.WebhookConfig, attemptNumber int, result *DeliverResult) (string, error) {
	log := h.log.With("delivery_id", delivery.ID, "webhook_id", cfg.ID, "attempt", attemptNumber)

	var outcome string
	if result.Success {
		outcome = "succeeded"
		log.Info("webhook delivery succeeded", "status_code", result.StatusCode, "duration_ms", result.DurationMs)
	} else if domain.IsRetryableHTTPStatus(result.StatusCode) && attemptNumber < maxWebhookRetries {
		outcome = "retry_scheduled"
		backoff := time.Duration(attemptNumber*attemptNumber) * 30 * time.Second
		nextAttemptAt := h.clock().Add(backoff)
		log.Warn("webhook delivery failed, scheduling retry",
			"status_code", result.StatusCode, "next_attempt_at", nextAttemptAt, "backoff", backoff)
		return outcome, h.emitDeliveryOutcome(ctx, delivery, cfg, result, outcome, &nextAttemptAt)
	} else {
		outcome = "failed"
		if attemptNumber >= maxWebhookRetries {
			log.Warn("webhook delivery max retries exceeded", "max_retries", maxWebhookRetries, "status_code", result.StatusCode)
		} else {
			log.Warn("webhook delivery terminally failed", "status_code", result.StatusCode)
		}
	}
	return outcome, h.emitDeliveryOutcome(ctx, delivery, cfg, result, outcome, nil)
}

func (h *ProcessHandler) emitDeliveryOutcome(ctx context.Context, delivery *domain.WebhookDelivery, cfg *domain.WebhookConfig, result *DeliverResult, outcome string, nextAttemptAt *time.Time) error {
	usesOutbox := h.outboxWriter != nil
	now := h.clock()

	return h.txManager.WithinTx(ctx, func(txCtx context.Context) error {
		switch outcome {
		case "succeeded":
			if err := h.deliveryWrite.MarkSucceeded(txCtx, delivery.ID, result.DeliveryResult); err != nil {
				return err
			}
		case "retry_scheduled":
			if err := h.deliveryWrite.ScheduleRetry(txCtx, delivery.ID, *nextAttemptAt); err != nil {
				return err
			}
		case "failed":
			if err := h.deliveryWrite.MarkFailed(txCtx, delivery.ID, result.DeliveryResult); err != nil {
				return err
			}
		}

		if err := h.attemptWrite.Create(txCtx, result.Attempt); err != nil {
			return err
		}

		if !usesOutbox {
			return nil
		}

		eventID, err := h.idGen()
		if err != nil {
			return err
		}

		switch outcome {
		case "succeeded":
			payload, _ := json.Marshal(webhookscontracts.DeliverySucceededPayload{
				DeliveryID:      delivery.ID,
				WorkspaceID:     delivery.WorkspaceID,
				WebhookID:       cfg.ID,
				SourceEventID:   delivery.SourceEventID,
				SourceEventType: delivery.SourceEventType,
				StatusCode:      result.StatusCode,
				DurationMs:      result.DurationMs,
			})
			return h.outboxWriter.Save(txCtx, ports.OutboxEvent{
				ID:            eventID,
				AggregateType: webhookscontracts.AggregateWebhookDelivery,
				AggregateID:   delivery.ID,
				EventType:     webhookscontracts.EventDeliverySucceededV1,
				Payload:       payload,
				WorkspaceID:   delivery.WorkspaceID,
				OccurredAt:    now,
			})
		case "retry_scheduled":
			var sc *int
			if result.StatusCode > 0 {
				sc = &result.StatusCode
			}
			payload, _ := json.Marshal(webhookscontracts.DeliveryRetryScheduledPayload{
				DeliveryID:      delivery.ID,
				WorkspaceID:     delivery.WorkspaceID,
				WebhookID:       cfg.ID,
				SourceEventID:   delivery.SourceEventID,
				SourceEventType: delivery.SourceEventType,
				NextAttemptAt:   nextAttemptAt.UTC().Format(time.RFC3339),
				AttemptNumber:   int64(delivery.AttemptCount),
				StatusCode:      sc,
				Error:           result.Error,
			})
			return h.outboxWriter.Save(txCtx, ports.OutboxEvent{
				ID:            eventID,
				AggregateType: webhookscontracts.AggregateWebhookDelivery,
				AggregateID:   delivery.ID,
				EventType:     webhookscontracts.EventDeliveryRetryScheduledV1,
				Payload:       payload,
				WorkspaceID:   delivery.WorkspaceID,
				OccurredAt:    now,
			})
		case "failed":
			failedPayload := webhookscontracts.DeliveryFailedPayload{
				DeliveryID:      delivery.ID,
				WorkspaceID:     delivery.WorkspaceID,
				WebhookID:       cfg.ID,
				SourceEventID:   delivery.SourceEventID,
				SourceEventType: delivery.SourceEventType,
				Error:           result.Error,
				DurationMs:      result.DurationMs,
			}
			if result.Attempt.StatusCode != nil {
				failedPayload.StatusCode = result.Attempt.StatusCode
			}
			payload, _ := json.Marshal(failedPayload)
			return h.outboxWriter.Save(txCtx, ports.OutboxEvent{
				ID:            eventID,
				AggregateType: webhookscontracts.AggregateWebhookDelivery,
				AggregateID:   delivery.ID,
				EventType:     webhookscontracts.EventDeliveryFailedV1,
				Payload:       payload,
				WorkspaceID:   delivery.WorkspaceID,
				OccurredAt:    now,
			})
		}
		return nil
	})
}
