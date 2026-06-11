package app

import (
	"context"
	"fmt"
	"log/slog"
	"time"

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

func (s *Service) ProcessDueDelivery(ctx context.Context, workspaceID, deliveryID string) error {
	delivery, err := s.deliveryRead.FindByID(ctx, workspaceID, deliveryID)
	if err != nil {
		return fmt.Errorf("find delivery: %w", err)
	}

	cfg, err := s.configRead.FindByID(ctx, delivery.WorkspaceID, delivery.WebhookID)
	if err != nil {
		return fmt.Errorf("find webhook config: %w", err)
	}

	if cfg.Status != domain.ConfigStatusActive {
		if err := s.deliveryWrite.MarkFailed(ctx, deliveryID, domain.DeliveryResult{
			Error: "webhook config disabled",
		}); err != nil {
			return fmt.Errorf("mark failed: %w", err)
		}
		return nil
	}

	attemptNumber := delivery.AttemptCount

	err = s.DeliverWebhook(ctx, DeliverWebhookInput{
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
		return s.handleDeliveryError(ctx, delivery, cfg, int(attemptNumber), err)
	}

	return nil
}

const maxWebhookRetries = 5

func (s *Service) handleDeliveryError(ctx context.Context, delivery *domain.WebhookDelivery, cfg *domain.WebhookConfig, attemptNumber int, deliveryErr error) error {
	log := s.log.With("delivery_id", delivery.ID, "webhook_id", cfg.ID, "attempt", attemptNumber)
	log.Error("webhook delivery failed", "error", deliveryErr)

	if attemptNumber >= maxWebhookRetries {
		log.Warn("webhook delivery max retries exceeded", "max_retries", maxWebhookRetries)
		return nil
	}

	backoff := time.Duration(attemptNumber*attemptNumber) * 30 * time.Second
	nextAttempt := s.clock().Add(backoff)

	if err := s.deliveryWrite.ScheduleRetry(ctx, delivery.ID, nextAttempt); err != nil {
		return err
	}

	log.Info("webhook delivery retry scheduled", "next_attempt_at", nextAttempt, "backoff", backoff)
	return nil
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
