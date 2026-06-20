package delivery

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	webhookscontracts "github.com/ninggiangboy/send-flow/backend/internal/modules/webhooks/contracts"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/webhooks/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/webhooks/ports"
)

type RetryOptions struct {
	DeliveryWrite   ports.DeliveryWriteRepository
	AttemptWrite    ports.AttemptWriteRepository
	ConfigWrite     ports.ConfigWriteRepository
	TxManager       ports.TransactionManager
	OutboxWriter    ports.OutboxWriter
	AccessChecker   ports.WorkspaceAccessChecker
	DeliverWebhookH *DeliverHandler
	IDGen           func() (string, error)
	Clock           func() time.Time
	Logger          *slog.Logger
}

type RetryCommand struct {
	WorkspaceID string
	UserID      string
	DeliveryID  string
}

type RetryHandler struct {
	deliveryWrite   ports.DeliveryWriteRepository
	attemptWrite    ports.AttemptWriteRepository
	configWrite     ports.ConfigWriteRepository
	txManager       ports.TransactionManager
	outboxWriter    ports.OutboxWriter
	accessChecker   ports.WorkspaceAccessChecker
	deliverWebhookH *DeliverHandler
	idGen           func() (string, error)
	clock           func() time.Time
	log             *slog.Logger
}

func NewRetry(opts RetryOptions) *RetryHandler {
	return &RetryHandler{
		deliveryWrite:   opts.DeliveryWrite,
		attemptWrite:    opts.AttemptWrite,
		configWrite:     opts.ConfigWrite,
		txManager:       opts.TxManager,
		outboxWriter:    opts.OutboxWriter,
		accessChecker:   opts.AccessChecker,
		deliverWebhookH: opts.DeliverWebhookH,
		idGen:           opts.IDGen,
		clock:           opts.Clock,
		log:             opts.Logger.With("usecase", "retry_webhook_delivery"),
	}
}

func (h *RetryHandler) Execute(ctx context.Context, cmd RetryCommand) error {
	log := h.log.With("workspace_id", cmd.WorkspaceID, "delivery_id", cmd.DeliveryID)

	if err := h.accessChecker.RequirePermission(ctx, cmd.WorkspaceID, cmd.UserID, "webhook.delivery.retry"); err != nil {
		return err
	}

	delivery, err := h.deliveryWrite.FindByID(ctx, cmd.WorkspaceID, cmd.DeliveryID)
	if err != nil {
		if errors.Is(err, domain.ErrDeliveryNotFound) {
			return err
		}
		log.Error("failed to find delivery for retry", "error", err)
		return err
	}

	if !domain.CanDeliveryBeRetried(delivery.Status) {
		return domain.ErrRetryConflict
	}

	cfg, err := h.configWrite.FindByID(ctx, cmd.WorkspaceID, delivery.WebhookID)
	if err != nil {
		if errors.Is(err, domain.ErrConfigNotFound) {
			return domain.ErrConfigNotFound
		}
		log.Error("failed to find webhook config for retry", "error", err)
		return err
	}

	if cfg.Status != domain.ConfigStatusActive {
		return domain.ErrConfigDisabled
	}

	now := h.clock()
	nextAttempt := delivery.AttemptCount + 1

	if err := h.deliveryWrite.MarkDelivering(ctx, cmd.DeliveryID, now); err != nil {
		if errors.Is(err, domain.ErrDeliveryNotFound) {
			return err
		}
		log.Error("failed to mark delivery as delivering for retry", "error", err)
		return err
	}

	result, err := h.deliverWebhookH.Execute(ctx, DeliverCommand{
		WorkspaceID:     cmd.WorkspaceID,
		WebhookID:       cfg.ID,
		DeliveryID:      cmd.DeliveryID,
		TargetURL:       cfg.TargetURL,
		SecretHash:      cfg.SecretHash,
		EventPayload:    delivery.EventPayloadJSON,
		SourceEventID:   delivery.SourceEventID,
		SourceEventType: delivery.SourceEventType,
		AttemptNumber:   nextAttempt,
	})
	if err != nil {
		log.Error("retry delivery failed", "error", err)
		return err
	}

	usesOutbox := h.outboxWriter != nil
	return h.txManager.WithinTx(ctx, func(txCtx context.Context) error {
		if result.Success {
			if err := h.deliveryWrite.MarkSucceeded(txCtx, cmd.DeliveryID, result.DeliveryResult); err != nil {
				return err
			}
			if err := h.attemptWrite.Create(txCtx, result.Attempt); err != nil {
				return err
			}
			if usesOutbox {
				eventID, err := h.idGen()
				if err != nil {
					return err
				}
				payload, _ := json.Marshal(webhookscontracts.DeliverySucceededPayload{
					DeliveryID:      cmd.DeliveryID,
					WorkspaceID:     cmd.WorkspaceID,
					WebhookID:       cfg.ID,
					SourceEventID:   delivery.SourceEventID,
					SourceEventType: delivery.SourceEventType,
					StatusCode:      result.StatusCode,
					DurationMs:      result.DurationMs,
				})
				return h.outboxWriter.Save(txCtx, ports.OutboxEvent{
					ID:            eventID,
					AggregateType: webhookscontracts.AggregateWebhookDelivery,
					AggregateID:   cmd.DeliveryID,
					EventType:     webhookscontracts.EventDeliverySucceededV1,
					Payload:       payload,
					WorkspaceID:   cmd.WorkspaceID,
					OccurredAt:    now,
				})
			}
			return nil
		}

		if err := h.deliveryWrite.MarkFailed(txCtx, cmd.DeliveryID, result.DeliveryResult); err != nil {
			return err
		}
		if err := h.attemptWrite.Create(txCtx, result.Attempt); err != nil {
			return err
		}
		if usesOutbox {
			eventID, err := h.idGen()
			if err != nil {
				return err
			}
			failedPayload := webhookscontracts.DeliveryFailedPayload{
				DeliveryID:      cmd.DeliveryID,
				WorkspaceID:     cmd.WorkspaceID,
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
				AggregateID:   cmd.DeliveryID,
				EventType:     webhookscontracts.EventDeliveryFailedV1,
				Payload:       payload,
				WorkspaceID:   cmd.WorkspaceID,
				OccurredAt:    now,
			})
		}
		return nil
	})
}
