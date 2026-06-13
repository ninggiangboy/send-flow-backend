package app

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

type Options struct {
	ConfigRead    ports.ConfigReadRepository
	ConfigWrite   ports.ConfigWriteRepository
	DeliveryRead  ports.DeliveryReadRepository
	DeliveryWrite ports.DeliveryWriteRepository
	AttemptWrite  ports.AttemptWriteRepository
	AttemptRead   ports.AttemptReadRepository
	TxManager     ports.TransactionManager
	OutboxWriter  ports.OutboxWriter
	Deliverer     ports.HTTPDeliverer
	AccessChecker ports.WorkspaceAccessChecker
	IDGen         func() (string, error)
	Clock         func() time.Time
	Logger        *slog.Logger
}

type Service struct {
	configRead    ports.ConfigReadRepository
	configWrite   ports.ConfigWriteRepository
	deliveryRead  ports.DeliveryReadRepository
	deliveryWrite ports.DeliveryWriteRepository
	attemptWrite  ports.AttemptWriteRepository
	attemptRead   ports.AttemptReadRepository
	txManager     ports.TransactionManager
	outboxWriter  ports.OutboxWriter
	deliverer     ports.HTTPDeliverer
	accessChecker ports.WorkspaceAccessChecker
	idGen         func() (string, error)
	clock         func() time.Time
	log           *slog.Logger
}

func (s *Service) ClaimDueDeliveries(ctx context.Context, limit int, now time.Time) ([]domain.WebhookDelivery, error) {
	return s.deliveryWrite.ClaimPendingDeliveries(ctx, limit, now)
}

func (s *Service) ProcessDueDelivery(ctx context.Context, workspaceID, deliveryID string) (string, error) {
	delivery, err := s.deliveryRead.FindByID(ctx, workspaceID, deliveryID)
	if err != nil {
		return "", fmt.Errorf("find delivery: %w", err)
	}

	cfg, err := s.configRead.FindByID(ctx, delivery.WorkspaceID, delivery.WebhookID)
	if err != nil {
		return "", fmt.Errorf("find webhook config: %w", err)
	}

	if cfg.Status != domain.ConfigStatusActive {
		if err := s.deliveryWrite.MarkFailed(ctx, deliveryID, domain.DeliveryResult{
			Error: "webhook config disabled",
		}); err != nil {
			return "", fmt.Errorf("mark failed: %w", err)
		}
		return "failed", nil
	}

	attemptNumber := int(delivery.AttemptCount)

	result, err := s.DeliverWebhook(ctx, DeliverWebhookInput{
		WorkspaceID:     delivery.WorkspaceID,
		WebhookID:       cfg.ID,
		DeliveryID:      deliveryID,
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

	return s.recordDeliveryOutcome(ctx, delivery, cfg, attemptNumber, result)
}

const maxWebhookRetries = 5

func (s *Service) recordDeliveryOutcome(ctx context.Context, delivery *domain.WebhookDelivery, cfg *domain.WebhookConfig, attemptNumber int, result *DeliverWebhookResult) (string, error) {
	log := s.log.With("delivery_id", delivery.ID, "webhook_id", cfg.ID, "attempt", attemptNumber)

	var outcome string
	if result.Success {
		outcome = "succeeded"
		log.Info("webhook delivery succeeded", "status_code", result.StatusCode, "duration_ms", result.DurationMs)
	} else if domain.IsRetryableHTTPStatus(result.StatusCode) && attemptNumber < maxWebhookRetries {
		outcome = "retry_scheduled"
		backoff := time.Duration(attemptNumber*attemptNumber) * 30 * time.Second
		nextAttemptAt := s.clock().Add(backoff)
		log.Warn("webhook delivery failed, scheduling retry",
			"status_code", result.StatusCode, "next_attempt_at", nextAttemptAt, "backoff", backoff)
		return outcome, s.emitDeliveryOutcome(ctx, delivery, cfg, result, outcome, &nextAttemptAt, nil)
	} else {
		outcome = "failed"
		if attemptNumber >= maxWebhookRetries {
			log.Warn("webhook delivery max retries exceeded", "max_retries", maxWebhookRetries, "status_code", result.StatusCode)
		} else {
			log.Warn("webhook delivery terminally failed", "status_code", result.StatusCode)
		}
	}
	return outcome, s.emitDeliveryOutcome(ctx, delivery, cfg, result, outcome, nil, nil)
}

func (s *Service) emitDeliveryOutcome(ctx context.Context, delivery *domain.WebhookDelivery, cfg *domain.WebhookConfig, result *DeliverWebhookResult, outcome string, nextAttemptAt *time.Time, _ error) error {
	usesOutbox := s.outboxWriter != nil
	now := s.clock()

	return s.txManager.WithinTx(ctx, func(txCtx context.Context) error {
		switch outcome {
		case "succeeded":
			if err := s.deliveryWrite.MarkSucceeded(txCtx, delivery.ID, result.DeliveryResult); err != nil {
				return err
			}
		case "retry_scheduled":
			if err := s.deliveryWrite.ScheduleRetry(txCtx, delivery.ID, *nextAttemptAt); err != nil {
				return err
			}
		case "failed":
			if err := s.deliveryWrite.MarkFailed(txCtx, delivery.ID, result.DeliveryResult); err != nil {
				return err
			}
		}

		if err := s.attemptWrite.Create(txCtx, result.Attempt); err != nil {
			return err
		}

		if !usesOutbox {
			return nil
		}

		eventID, err := s.idGen()
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
			return s.outboxWriter.Save(txCtx, ports.OutboxEvent{
				ID:            eventID,
				AggregateType: "webhook_delivery",
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
			return s.outboxWriter.Save(txCtx, ports.OutboxEvent{
				ID:            eventID,
				AggregateType: "webhook_delivery",
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
			return s.outboxWriter.Save(txCtx, ports.OutboxEvent{
				ID:            eventID,
				AggregateType: "webhook_delivery",
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

func NewService(opts Options) *Service {
	if opts.Clock == nil {
		opts.Clock = time.Now
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	return &Service{
		configRead:    opts.ConfigRead,
		configWrite:   opts.ConfigWrite,
		deliveryRead:  opts.DeliveryRead,
		deliveryWrite: opts.DeliveryWrite,
		attemptWrite:  opts.AttemptWrite,
		attemptRead:   opts.AttemptRead,
		txManager:     opts.TxManager,
		outboxWriter:  opts.OutboxWriter,
		deliverer:     opts.Deliverer,
		accessChecker: opts.AccessChecker,
		idGen:         opts.IDGen,
		clock:         opts.Clock,
		log:           opts.Logger.With("module", "webhooks"),
	}
}
