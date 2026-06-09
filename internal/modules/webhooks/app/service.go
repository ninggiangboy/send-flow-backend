package app

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
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

type CreateConfigInput struct {
	WorkspaceID   string
	UserID        string
	Name          string
	TargetURL     string
	Subscriptions []string
	Now           time.Time
}

type ConfigResult struct {
	Config    domain.WebhookConfig
	RawSecret string
}

func (s *Service) CreateWebhookConfig(ctx context.Context, input CreateConfigInput) (*ConfigResult, error) {
	log := s.log.With("usecase", "create_webhook_config", "workspace_id", input.WorkspaceID)
	if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, "webhook.manage"); err != nil {
		if errors.Is(err, domain.ErrManageDenied) || errors.Is(err, domain.ErrDeliveryReadDenied) || errors.Is(err, domain.ErrDeliveryRetryDenied) {
			return nil, err
		}
		return nil, err
	}

	cfg := domain.WebhookConfig{
		Name:          input.Name,
		WorkspaceID:   input.WorkspaceID,
		TargetURL:     input.TargetURL,
		Subscriptions: input.Subscriptions,
		Status:        domain.ConfigStatusActive,
	}
	if err := domain.ValidateWebhookConfig(cfg); err != nil {
		log.Warn("invalid webhook config", "error", err)
		return nil, err
	}

	id, err := s.idGen()
	if err != nil {
		return nil, err
	}

	rawSecret, hash, hint, err := GenerateSecret()
	if err != nil {
		log.Error("failed to generate secret", "error", err)
		return nil, err
	}

	now := s.clock()
	cfg.ID = id
	cfg.SecretHash = hash
	cfg.SecretHint = hint
	cfg.Version = 1
	cfg.CreatedByUserID = input.UserID
	cfg.CreatedAt = now
	cfg.UpdatedAt = now

	if err := s.txManager.RunInTransaction(ctx, func(txCtx context.Context) error {
		if err := s.configWrite.Create(txCtx, cfg); err != nil {
			return err
		}
		if s.outboxWriter != nil {
			eventID, err := s.idGen()
			if err != nil {
				return err
			}
			payload, err := json.Marshal(webhookscontracts.ConfigCreatedPayload{
				ConfigID:    id,
				WorkspaceID: input.WorkspaceID,
				Name:        input.Name,
				TargetURL:   input.TargetURL,
				Status:      string(cfg.Status),
				Subscribed:  input.Subscriptions,
			})
			if err != nil {
				return err
			}
			if err := s.outboxWriter.Save(txCtx, ports.OutboxEvent{
				ID:            eventID,
				AggregateType: "webhook_config",
				AggregateID:   id,
				EventType:     webhookscontracts.EventConfigCreatedV1,
				Payload:       payload,
				WorkspaceID:   input.WorkspaceID,
				OccurredAt:    now,
			}); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		if errors.Is(err, domain.ErrConfigNameConflict) {
			return nil, err
		}
		log.Error("failed to create webhook config", "error", err)
		return nil, err
	}

	log.Info("webhook config created", "webhook_id", id)
	return &ConfigResult{Config: cfg, RawSecret: rawSecret}, nil
}

type ListConfigsInput struct {
	WorkspaceID string
	UserID      string
}

func (s *Service) ListWebhookConfigs(ctx context.Context, input ListConfigsInput) ([]domain.WebhookConfig, error) {
	log := s.log.With("usecase", "list_webhook_configs", "workspace_id", input.WorkspaceID)
	if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, "webhook.manage"); err != nil {
		if errors.Is(err, domain.ErrManageDenied) || errors.Is(err, domain.ErrDeliveryReadDenied) || errors.Is(err, domain.ErrDeliveryRetryDenied) {
			return nil, err
		}
		return nil, err
	}

	configs, err := s.configRead.ListByWorkspace(ctx, input.WorkspaceID)
	if err != nil {
		log.Error("failed to list webhook configs", "error", err)
		return nil, err
	}

	return configs, nil
}

type UpdateConfigInput struct {
	WorkspaceID   string
	UserID        string
	WebhookID     string
	Name          *string
	TargetURL     *string
	Subscriptions []string
	Status        *string
	Now           time.Time
}

func (s *Service) UpdateWebhookConfig(ctx context.Context, input UpdateConfigInput) (*ConfigResult, error) {
	log := s.log.With("usecase", "update_webhook_config", "workspace_id", input.WorkspaceID, "webhook_id", input.WebhookID)
	if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, "webhook.manage"); err != nil {
		if errors.Is(err, domain.ErrManageDenied) || errors.Is(err, domain.ErrDeliveryReadDenied) || errors.Is(err, domain.ErrDeliveryRetryDenied) {
			return nil, err
		}
		return nil, err
	}

	cfg, err := s.configRead.FindByID(ctx, input.WorkspaceID, input.WebhookID)
	if err != nil {
		if errors.Is(err, domain.ErrConfigNotFound) {
			return nil, err
		}
		log.Error("failed to find webhook config", "error", err)
		return nil, err
	}

	if input.Name != nil {
		cfg.Name = *input.Name
	}
	if input.TargetURL != nil {
		cfg.TargetURL = *input.TargetURL
	}
	if input.Subscriptions != nil {
		cfg.Subscriptions = input.Subscriptions
	}
	if input.Status != nil {
		cfg.Status = domain.ConfigStatus(*input.Status)
	}

	if err := domain.ValidateWebhookConfig(*cfg); err != nil {
		log.Warn("invalid webhook config update", "error", err)
		return nil, err
	}

	cfg.Version++
	cfg.UpdatedAt = input.Now

	if err := s.configWrite.Update(ctx, *cfg); err != nil {
		if errors.Is(err, domain.ErrConfigNameConflict) {
			return nil, err
		}
		if errors.Is(err, domain.ErrRotateConflict) {
			return nil, domain.ErrRotateConflict
		}
		log.Error("failed to update webhook config", "error", err)
		return nil, err
	}

	if s.outboxWriter != nil {
		eventID, err := s.idGen()
		if err != nil {
			return nil, err
		}
		payload, err := json.Marshal(webhookscontracts.ConfigUpdatedPayload{
			ConfigID:    cfg.ID,
			WorkspaceID: cfg.WorkspaceID,
			Version:     cfg.Version,
			Status:      string(cfg.Status),
			Subscribed:  cfg.Subscriptions,
		})
		if err != nil {
			return nil, err
		}
		if err := s.outboxWriter.Save(ctx, ports.OutboxEvent{
			ID:            eventID,
			AggregateType: "webhook_config",
			AggregateID:   cfg.ID,
			EventType:     webhookscontracts.EventConfigUpdatedV1,
			Payload:       payload,
			WorkspaceID:   cfg.WorkspaceID,
			OccurredAt:    input.Now,
		}); err != nil {
			log.Error("failed to write outbox event", "error", err)
		}
	}

	log.Info("webhook config updated", "version", cfg.Version)
	return &ConfigResult{Config: *cfg}, nil
}

type DisableConfigInput struct {
	WorkspaceID string
	UserID      string
	WebhookID   string
}

func (s *Service) DisableWebhookConfig(ctx context.Context, input DisableConfigInput) error {
	log := s.log.With("usecase", "disable_webhook_config", "workspace_id", input.WorkspaceID, "webhook_id", input.WebhookID)
	if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, "webhook.manage"); err != nil {
		if errors.Is(err, domain.ErrManageDenied) || errors.Is(err, domain.ErrDeliveryReadDenied) || errors.Is(err, domain.ErrDeliveryRetryDenied) {
			return err
		}
		return err
	}

	now := s.clock()
	if err := s.configWrite.Disable(ctx, input.WorkspaceID, input.WebhookID, now); err != nil {
		if errors.Is(err, domain.ErrConfigNotFound) {
			return err
		}
		log.Error("failed to disable webhook config", "error", err)
		return err
	}

	if s.outboxWriter != nil {
		eventID, err := s.idGen()
		if err != nil {
			return err
		}
		payload, err := json.Marshal(webhookscontracts.ConfigDisabledPayload{
			ConfigID:    input.WebhookID,
			WorkspaceID: input.WorkspaceID,
			DisabledAt:  now.UTC().Format(time.RFC3339),
		})
		if err != nil {
			return err
		}
		if err := s.outboxWriter.Save(ctx, ports.OutboxEvent{
			ID:            eventID,
			AggregateType: "webhook_config",
			AggregateID:   input.WebhookID,
			EventType:     webhookscontracts.EventConfigDisabledV1,
			Payload:       payload,
			WorkspaceID:   input.WorkspaceID,
			OccurredAt:    now,
		}); err != nil {
			log.Error("failed to write outbox event", "error", err)
		}
	}

	log.Info("webhook config disabled")
	return nil
}

type RotateSecretInput struct {
	WorkspaceID string
	UserID      string
	WebhookID   string
}

type RotateSecretResult struct {
	RawSecret string
	Hint      string
	Version   int64
}

func (s *Service) RotateWebhookSecret(ctx context.Context, input RotateSecretInput) (*RotateSecretResult, error) {
	log := s.log.With("usecase", "rotate_webhook_secret", "workspace_id", input.WorkspaceID, "webhook_id", input.WebhookID)
	if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, "webhook.manage"); err != nil {
		if errors.Is(err, domain.ErrManageDenied) || errors.Is(err, domain.ErrDeliveryReadDenied) || errors.Is(err, domain.ErrDeliveryRetryDenied) {
			return nil, err
		}
		return nil, err
	}

	cfg, err := s.configRead.FindByID(ctx, input.WorkspaceID, input.WebhookID)
	if err != nil {
		if errors.Is(err, domain.ErrConfigNotFound) {
			return nil, err
		}
		log.Error("failed to find webhook config for rotate", "error", err)
		return nil, err
	}

	rawSecret, hash, hint, err := GenerateSecret()
	if err != nil {
		log.Error("failed to generate secret", "error", err)
		return nil, err
	}

	cfg.SecretHash = hash
	cfg.SecretHint = hint
	cfg.Version++
	cfg.UpdatedAt = s.clock()

	if err := s.configWrite.Update(ctx, *cfg); err != nil {
		if errors.Is(err, domain.ErrRotateConflict) {
			return nil, err
		}
		log.Error("failed to update config with new secret", "error", err)
		return nil, err
	}

	if s.outboxWriter != nil {
		eventID, err := s.idGen()
		if err != nil {
			return nil, err
		}
		payload, err := json.Marshal(webhookscontracts.SecretRotatedPayload{
			ConfigID:    cfg.ID,
			WorkspaceID: cfg.WorkspaceID,
			Version:     cfg.Version,
			SecretHint:  hint,
		})
		if err != nil {
			return nil, err
		}
		if err := s.outboxWriter.Save(ctx, ports.OutboxEvent{
			ID:            eventID,
			AggregateType: "webhook_config",
			AggregateID:   cfg.ID,
			EventType:     webhookscontracts.EventSecretRotatedV1,
			Payload:       payload,
			WorkspaceID:   cfg.WorkspaceID,
			OccurredAt:    cfg.UpdatedAt,
		}); err != nil {
			log.Error("failed to write outbox event", "error", err)
		}
	}

	log.Info("webhook secret rotated", "version", cfg.Version)
	return &RotateSecretResult{RawSecret: rawSecret, Hint: hint, Version: cfg.Version}, nil
}

type DeliverWebhookInput struct {
	WorkspaceID     string
	WebhookID       string
	DeliveryID      string
	TargetURL       string
	SecretHash      string
	EventPayload    map[string]any
	SourceEventID   string
	SourceEventType string
	AttemptNumber   int64
}

func (s *Service) DeliverWebhook(ctx context.Context, input DeliverWebhookInput) error {
	now := s.clock()
	timestamp := fmt.Sprintf("%d", now.Unix())

	payloadJSON, err := json.Marshal(input.EventPayload)
	if err != nil {
		return err
	}

	signature := SignPayloadRaw(payloadJSON, timestamp, input.SecretHash)

	deliveryReq := ports.DeliveryHTTPRequest{
		URL:             input.TargetURL,
		Body:            payloadJSON,
		SignatureHeader: "Sendflow-Signature",
		SignatureValue:  signature,
		TimestampHeader: "Sendflow-Timestamp",
		TimestampValue:  timestamp,
		EventIDHeader:   "Sendflow-Event-ID",
		EventIDValue:    input.SourceEventID,
	}

	resp, err := s.deliverer.Deliver(ctx, deliveryReq)
	if err != nil {
		return err
	}

	attemptID, err := s.idGen()
	if err != nil {
		return err
	}

	isSuccess := resp.StatusCode >= 200 && resp.StatusCode < 300
	status := string(domain.DeliveryStatusFailed)
	if isSuccess {
		status = string(domain.DeliveryStatusSucceeded)
	}

	attempt := domain.WebhookDeliveryAttempt{
		ID:              attemptID,
		DeliveryID:      input.DeliveryID,
		AttemptNumber:   input.AttemptNumber,
		Status:          status,
		StatusCode:      &resp.StatusCode,
		Error:           domain.SanitizeError(resp.Error),
		DurationMs:      resp.DurationMs,
		RequestHeaders:  map[string]string{},
		ResponseHeaders: domain.SanitizeHeaders(resp.Headers),
		AttemptedAt:     now,
	}
	if resp.StatusCode == 0 {
		attempt.StatusCode = nil
	}

	deliveryResult := domain.DeliveryResult{
		StatusCode:      resp.StatusCode,
		DurationMs:      resp.DurationMs,
		Error:           domain.SanitizeError(resp.Error),
		RequestHeaders:  map[string]string{},
		ResponseHeaders: domain.SanitizeHeaders(resp.Headers),
	}

	if isSuccess {
		if err := s.deliveryWrite.MarkSucceeded(ctx, input.DeliveryID, deliveryResult); err != nil {
			return err
		}
	} else {
		if err := s.deliveryWrite.MarkFailed(ctx, input.DeliveryID, deliveryResult); err != nil {
			return err
		}
	}

	if err := s.attemptWrite.Create(ctx, attempt); err != nil {
		return err
	}

	eventID, err := s.idGen()
	if err != nil {
		return err
	}

	if isSuccess {
		payload, _ := json.Marshal(webhookscontracts.DeliverySucceededPayload{
			DeliveryID:      input.DeliveryID,
			WorkspaceID:     input.WorkspaceID,
			WebhookID:       input.WebhookID,
			SourceEventID:   input.SourceEventID,
			SourceEventType: input.SourceEventType,
			StatusCode:      resp.StatusCode,
			DurationMs:      resp.DurationMs,
		})
		if s.outboxWriter != nil {
			s.outboxWriter.Save(ctx, ports.OutboxEvent{
				ID:            eventID,
				AggregateType: "webhook_delivery",
				AggregateID:   input.DeliveryID,
				EventType:     webhookscontracts.EventDeliverySucceededV1,
				Payload:       payload,
				WorkspaceID:   input.WorkspaceID,
				OccurredAt:    now,
			})
		}
	} else {
		failedPayload := webhookscontracts.DeliveryFailedPayload{
			DeliveryID:      input.DeliveryID,
			WorkspaceID:     input.WorkspaceID,
			WebhookID:       input.WebhookID,
			SourceEventID:   input.SourceEventID,
			SourceEventType: input.SourceEventType,
			Error:           domain.SanitizeError(resp.Error),
			DurationMs:      resp.DurationMs,
		}
		if resp.StatusCode != 0 {
			sc := resp.StatusCode
			failedPayload.StatusCode = &sc
		}
		payload, _ := json.Marshal(failedPayload)
		if s.outboxWriter != nil {
			s.outboxWriter.Save(ctx, ports.OutboxEvent{
				ID:            eventID,
				AggregateType: "webhook_delivery",
				AggregateID:   input.DeliveryID,
				EventType:     webhookscontracts.EventDeliveryFailedV1,
				Payload:       payload,
				WorkspaceID:   input.WorkspaceID,
				OccurredAt:    now,
			})
		}
	}

	return nil
}

type HandleSourceEventInput struct {
	RawPayload []byte
}

func (s *Service) HandleSourceEvent(ctx context.Context, input HandleSourceEventInput) error {
	if len(input.RawPayload) == 0 {
		return nil
	}

	var envelope struct {
		EventID     string          `json:"event_id"`
		EventType   string          `json:"event_type"`
		WorkspaceID string          `json:"workspace_id"`
		OccurredAt  time.Time       `json:"occurred_at"`
		Payload     json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(input.RawPayload, &envelope); err != nil {
		return &NonRetryableError{Err: fmt.Errorf("malformed envelope: %w", err)}
	}

	if envelope.EventType == "" || envelope.WorkspaceID == "" {
		return &NonRetryableError{Err: errors.New("event missing event_type or workspace_id")}
	}

	supported := domain.AllowedSubscriptionEvents[envelope.EventType]
	if !supported {
		return nil
	}

	log := s.log.With("usecase", "handle_source_event", "workspace_id", envelope.WorkspaceID, "event_type", envelope.EventType, "event_id", envelope.EventID)

	configs, err := s.configRead.ListSubscribed(ctx, envelope.WorkspaceID, envelope.EventType)
	if err != nil {
		log.Error("failed to list subscribed configs", "error", err)
		return err
	}

	now := s.clock()

	for _, cfg := range configs {
		deliveryID, err := s.idGen()
		if err != nil {
			return err
		}

		eventPayload := map[string]any{
			"id":           envelope.EventID,
			"type":         envelope.EventType,
			"workspace_id": envelope.WorkspaceID,
			"occurred_at":  envelope.OccurredAt.UTC().Format(time.RFC3339),
			"data":         json.RawMessage(envelope.Payload),
		}

		delivery := domain.WebhookDelivery{
			ID:               deliveryID,
			WorkspaceID:      envelope.WorkspaceID,
			WebhookID:        cfg.ID,
			SourceEventID:    envelope.EventID,
			SourceEventType:  envelope.EventType,
			Status:           domain.DeliveryStatusPending,
			TargetURL:        cfg.TargetURL,
			AttemptCount:     0,
			RequestHeaders:   map[string]string{},
			ResponseHeaders:  map[string]string{},
			EventPayloadJSON: eventPayload,
			CreatedAt:        now,
			UpdatedAt:        now,
		}

		if err := s.deliveryWrite.Create(ctx, delivery); err != nil {
			log.Error("failed to create delivery record", "error", err, "webhook_id", cfg.ID)
			continue
		}

		if cfg.Status != domain.ConfigStatusActive {
			log.Debug("webhook config disabled, skipping delivery", "webhook_id", cfg.ID)
			continue
		}

		if err := s.deliveryWrite.MarkDelivering(ctx, deliveryID, now); err != nil {
			log.Error("failed to mark delivery as delivering", "error", err, "delivery_id", deliveryID)
			continue
		}

		if err := s.DeliverWebhook(ctx, DeliverWebhookInput{
			WorkspaceID:     envelope.WorkspaceID,
			WebhookID:       cfg.ID,
			DeliveryID:      deliveryID,
			TargetURL:       cfg.TargetURL,
			SecretHash:      cfg.SecretHash,
			EventPayload:    eventPayload,
			SourceEventID:   envelope.EventID,
			SourceEventType: envelope.EventType,
			AttemptNumber:   1,
		}); err != nil {
			log.Error("failed to deliver webhook", "error", err, "delivery_id", deliveryID)
		}
	}

	log.Info("source event processed", "matched_configs", len(configs))
	return nil
}

type ListDeliveriesInput struct {
	WorkspaceID string
	UserID      string
	WebhookID   string
	Status      string
	EventType   string
	From        *time.Time
	To          *time.Time
	Limit       int
	Cursor      string
}

type ListDeliveriesResult struct {
	Deliveries []domain.WebhookDelivery
	NextCursor string
}

func (s *Service) ListWebhookDeliveries(ctx context.Context, input ListDeliveriesInput) (*ListDeliveriesResult, error) {
	log := s.log.With("usecase", "list_webhook_deliveries", "workspace_id", input.WorkspaceID)
	if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, "webhook.delivery.read"); err != nil {
		if errors.Is(err, domain.ErrDeliveryReadDenied) || errors.Is(err, domain.ErrManageDenied) || errors.Is(err, domain.ErrDeliveryRetryDenied) {
			return nil, err
		}
		return nil, err
	}

	filter := ports.DeliveryFilter{
		WebhookID: input.WebhookID,
		Status:    input.Status,
		EventType: input.EventType,
		From:      input.From,
		To:        input.To,
		Limit:     input.Limit,
		Cursor:    input.Cursor,
	}

	deliveries, cursor, err := s.deliveryRead.ListByWorkspace(ctx, input.WorkspaceID, filter)
	if err != nil {
		log.Error("failed to list deliveries", "error", err)
		return nil, err
	}

	return &ListDeliveriesResult{Deliveries: deliveries, NextCursor: cursor}, nil
}

type GetDeliveryInput struct {
	WorkspaceID string
	UserID      string
	DeliveryID  string
}

func (s *Service) GetWebhookDelivery(ctx context.Context, input GetDeliveryInput) (*domain.WebhookDelivery, error) {
	log := s.log.With("usecase", "get_webhook_delivery", "workspace_id", input.WorkspaceID, "delivery_id", input.DeliveryID)
	if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, "webhook.delivery.read"); err != nil {
		if errors.Is(err, domain.ErrDeliveryReadDenied) || errors.Is(err, domain.ErrManageDenied) || errors.Is(err, domain.ErrDeliveryRetryDenied) {
			return nil, err
		}
		return nil, err
	}

	delivery, err := s.deliveryRead.FindByID(ctx, input.WorkspaceID, input.DeliveryID)
	if err != nil {
		if errors.Is(err, domain.ErrDeliveryNotFound) {
			return nil, err
		}
		log.Error("failed to find delivery", "error", err)
		return nil, err
	}

	attempts, err := s.attemptRead.ListByDelivery(ctx, input.DeliveryID)
	if err != nil {
		log.Error("failed to list delivery attempts", "error", err)
	}
	delivery.Attempts = attempts

	return delivery, nil
}

type RetryDeliveryInput struct {
	WorkspaceID string
	UserID      string
	DeliveryID  string
}

func (s *Service) RetryWebhookDelivery(ctx context.Context, input RetryDeliveryInput) error {
	log := s.log.With("usecase", "retry_webhook_delivery", "workspace_id", input.WorkspaceID, "delivery_id", input.DeliveryID)
	if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, "webhook.delivery.retry"); err != nil {
		if errors.Is(err, domain.ErrDeliveryRetryDenied) || errors.Is(err, domain.ErrManageDenied) || errors.Is(err, domain.ErrDeliveryReadDenied) {
			return err
		}
		return err
	}

	delivery, err := s.deliveryRead.FindByID(ctx, input.WorkspaceID, input.DeliveryID)
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

	cfg, err := s.configRead.FindByID(ctx, input.WorkspaceID, delivery.WebhookID)
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

	now := s.clock()
	nextAttempt := delivery.AttemptCount + 1

	if err := s.deliveryWrite.MarkDelivering(ctx, input.DeliveryID, now); err != nil {
		if errors.Is(err, domain.ErrDeliveryNotFound) {
			return err
		}
		log.Error("failed to mark delivery as delivering for retry", "error", err)
		return err
	}

	if err := s.DeliverWebhook(ctx, DeliverWebhookInput{
		WorkspaceID:     input.WorkspaceID,
		WebhookID:       cfg.ID,
		DeliveryID:      input.DeliveryID,
		TargetURL:       cfg.TargetURL,
		SecretHash:      cfg.SecretHash,
		EventPayload:    delivery.EventPayloadJSON,
		SourceEventID:   delivery.SourceEventID,
		SourceEventType: delivery.SourceEventType,
		AttemptNumber:   nextAttempt,
	}); err != nil {
		log.Error("retry delivery failed", "error", err)
		return err
	}

	log.Info("webhook delivery retried", "attempt", nextAttempt)
	return nil
}

func GenerateSecret() (raw, signingKey, hint string, err error) {
	rawBytes := make([]byte, 32)
	if _, err := rand.Read(rawBytes); err != nil {
		return "", "", "", err
	}
	raw = hex.EncodeToString(rawBytes)

	hint = raw[:8] + "..."
	return raw, raw, hint, nil
}

func SignPayloadRaw(payload []byte, timestamp, signingKey string) string {
	mac := hmac.New(sha256.New, []byte(signingKey))
	mac.Write([]byte(timestamp))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

type NonRetryableError struct {
	Err error
}

func (e *NonRetryableError) Error() string {
	return "non-retryable: " + e.Err.Error()
}

func (e *NonRetryableError) Unwrap() error {
	return e.Err
}
