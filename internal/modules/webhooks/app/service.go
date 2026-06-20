package app

import (
	"context"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/webhooks/app/config"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/webhooks/app/delivery"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/webhooks/app/source"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/webhooks/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/webhooks/ports"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/events"
)

// Re-exported from source package for worker compatibility.
var (
	ErrUnsupportedEventType = source.ErrUnsupportedEventType
	ErrMalformedPayload     = source.ErrMalformedPayload
)

type MappedSourceEvent = source.MappedSourceEvent

func MapEnvelopeToSourceEvent(envelope events.Envelope) (*MappedSourceEvent, error) {
	return source.MapEnvelopeToSourceEvent(envelope)
}

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
	listWebhookConfigsH   *config.ListHandler
	createWebhookConfigH  *config.CreateHandler
	updateWebhookConfigH  *config.UpdateHandler
	disableWebhookConfigH *config.DisableHandler
	rotateSecretH         *config.RotateHandler
	deliverWebhookH       *delivery.DeliverHandler
	processDueDeliveryH   *delivery.ProcessHandler
	retryWebhookDeliveryH *delivery.RetryHandler
	listDeliveriesH       *delivery.ListHandler
	getDeliveryH          *delivery.GetHandler
	handleSourceEventH    *source.Handler
	deliveryWrite         ports.DeliveryWriteRepository
}

func NewService(opts Options) *Service {
	if opts.Clock == nil {
		opts.Clock = time.Now
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	logger := opts.Logger.With("module", "webhooks")

	deliverWebhookH := delivery.NewDeliver(delivery.DeliverOptions{
		Deliverer: opts.Deliverer,
		IDGen:     opts.IDGen,
		Clock:     opts.Clock,
		Logger:    logger,
	})

	return &Service{
		listWebhookConfigsH: config.NewList(config.ListOptions{
			ConfigRead:    opts.ConfigRead,
			AccessChecker: opts.AccessChecker,
			Logger:        logger,
		}),
		createWebhookConfigH: config.NewCreate(config.CreateOptions{
			ConfigWrite:   opts.ConfigWrite,
			TxManager:     opts.TxManager,
			OutboxWriter:  opts.OutboxWriter,
			AccessChecker: opts.AccessChecker,
			IDGen:         opts.IDGen,
			Clock:         opts.Clock,
			Logger:        logger,
		}),
		updateWebhookConfigH: config.NewUpdate(config.UpdateOptions{
			ConfigWrite:   opts.ConfigWrite,
			TxManager:     opts.TxManager,
			OutboxWriter:  opts.OutboxWriter,
			AccessChecker: opts.AccessChecker,
			IDGen:         opts.IDGen,
			Clock:         opts.Clock,
			Logger:        logger,
		}),
		disableWebhookConfigH: config.NewDisable(config.DisableOptions{
			ConfigWrite:   opts.ConfigWrite,
			TxManager:     opts.TxManager,
			OutboxWriter:  opts.OutboxWriter,
			AccessChecker: opts.AccessChecker,
			IDGen:         opts.IDGen,
			Clock:         opts.Clock,
			Logger:        logger,
		}),
		rotateSecretH: config.NewRotate(config.RotateOptions{
			ConfigWrite:   opts.ConfigWrite,
			TxManager:     opts.TxManager,
			OutboxWriter:  opts.OutboxWriter,
			AccessChecker: opts.AccessChecker,
			IDGen:         opts.IDGen,
			Clock:         opts.Clock,
			Logger:        logger,
		}),
		deliverWebhookH: deliverWebhookH,
		processDueDeliveryH: delivery.NewProcess(delivery.ProcessOptions{
			DeliveryWrite:   opts.DeliveryWrite,
			AttemptWrite:    opts.AttemptWrite,
			ConfigWrite:     opts.ConfigWrite,
			TxManager:       opts.TxManager,
			OutboxWriter:    opts.OutboxWriter,
			DeliverWebhookH: deliverWebhookH,
			IDGen:           opts.IDGen,
			Clock:           opts.Clock,
			Logger:          logger,
		}),
		retryWebhookDeliveryH: delivery.NewRetry(delivery.RetryOptions{
			DeliveryWrite:   opts.DeliveryWrite,
			AttemptWrite:    opts.AttemptWrite,
			ConfigWrite:     opts.ConfigWrite,
			TxManager:       opts.TxManager,
			OutboxWriter:    opts.OutboxWriter,
			AccessChecker:   opts.AccessChecker,
			DeliverWebhookH: deliverWebhookH,
			IDGen:           opts.IDGen,
			Clock:           opts.Clock,
			Logger:          logger,
		}),
		listDeliveriesH: delivery.NewList(delivery.ListOptions{
			DeliveryRead:  opts.DeliveryRead,
			AccessChecker: opts.AccessChecker,
			Logger:        logger,
		}),
		getDeliveryH: delivery.NewGet(delivery.GetOptions{
			DeliveryRead:  opts.DeliveryRead,
			AttemptRead:   opts.AttemptRead,
			AccessChecker: opts.AccessChecker,
			Logger:        logger,
		}),
		handleSourceEventH: source.New(source.Options{
			ConfigRead:    opts.ConfigRead,
			DeliveryWrite: opts.DeliveryWrite,
			IDGen:         opts.IDGen,
			Clock:         opts.Clock,
			Logger:        logger,
		}),
		deliveryWrite: opts.DeliveryWrite,
	}
}

// Shared input/output types for external callers.

type ConfigResult struct {
	ID              string     `json:"id"`
	WorkspaceID     string     `json:"workspace_id"`
	Name            string     `json:"name"`
	TargetURL       string     `json:"target_url"`
	Status          string     `json:"status"`
	Subscriptions   []string   `json:"subscriptions"`
	SecretHint      string     `json:"secret_hint"`
	RawSecret       string     `json:"raw_secret,omitempty"`
	Version         int64      `json:"version"`
	CreatedByUserID string     `json:"created_by_user_id"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	DisabledAt      *time.Time `json:"disabled_at,omitempty"`
}

func configToResult(cfg *domain.WebhookConfig, rawSecret string) ConfigResult {
	r := ConfigResult{
		ID:              cfg.ID,
		WorkspaceID:     cfg.WorkspaceID,
		Name:            cfg.Name,
		TargetURL:       cfg.TargetURL,
		Status:          string(cfg.Status),
		Subscriptions:   cfg.Subscriptions,
		SecretHint:      cfg.SecretHint,
		RawSecret:       rawSecret,
		Version:         cfg.Version,
		CreatedByUserID: cfg.CreatedByUserID,
		CreatedAt:       cfg.CreatedAt,
		UpdatedAt:       cfg.UpdatedAt,
		DisabledAt:      cfg.DisabledAt,
	}
	return r
}

type CreateConfigInput struct {
	WorkspaceID   string
	UserID        string
	Name          string
	TargetURL     string
	Subscriptions []string
	Now           time.Time
}

type ListConfigsInput struct {
	WorkspaceID string
	UserID      string
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

type DisableConfigInput struct {
	WorkspaceID string
	UserID      string
	WebhookID   string
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

type GetDeliveryInput struct {
	WorkspaceID string
	UserID      string
	DeliveryID  string
}

type RetryDeliveryInput struct {
	WorkspaceID string
	UserID      string
	DeliveryID  string
}

type HandleSourceEventInput struct {
	RawPayload []byte
}

// Facade methods.

func (s *Service) CreateWebhookConfig(ctx context.Context, input CreateConfigInput) (*ConfigResult, error) {
	result, err := s.createWebhookConfigH.Execute(ctx, config.CreateCommand{
		WorkspaceID:   input.WorkspaceID,
		UserID:        input.UserID,
		Name:          input.Name,
		TargetURL:     input.TargetURL,
		Subscriptions: input.Subscriptions,
	})
	if err != nil {
		return nil, err
	}

	r := configToResult(&result.Config, result.RawSecret)
	return &r, nil
}

func (s *Service) ListWebhookConfigs(ctx context.Context, input ListConfigsInput) ([]ConfigResult, error) {
	configs, err := s.listWebhookConfigsH.Execute(ctx, config.ListCommand{
		WorkspaceID: input.WorkspaceID,
		UserID:      input.UserID,
	})
	if err != nil {
		return nil, err
	}

	results := make([]ConfigResult, len(configs))
	for i, cfg := range configs {
		results[i] = configToResult(&cfg, "")
	}
	return results, nil
}

func (s *Service) UpdateWebhookConfig(ctx context.Context, input UpdateConfigInput) (*ConfigResult, error) {
	cfg, err := s.updateWebhookConfigH.Execute(ctx, config.UpdateCommand{
		WorkspaceID:   input.WorkspaceID,
		UserID:        input.UserID,
		WebhookID:     input.WebhookID,
		Name:          input.Name,
		TargetURL:     input.TargetURL,
		Subscriptions: input.Subscriptions,
		Status:        input.Status,
	})
	if err != nil {
		return nil, err
	}

	r := configToResult(cfg, "")
	return &r, nil
}

func (s *Service) DisableWebhookConfig(ctx context.Context, input DisableConfigInput) error {
	return s.disableWebhookConfigH.Execute(ctx, config.DisableCommand{
		WorkspaceID: input.WorkspaceID,
		UserID:      input.UserID,
		WebhookID:   input.WebhookID,
	})
}

func (s *Service) RotateWebhookSecret(ctx context.Context, input RotateSecretInput) (*RotateSecretResult, error) {
	result, err := s.rotateSecretH.Execute(ctx, config.RotateCommand{
		WorkspaceID: input.WorkspaceID,
		UserID:      input.UserID,
		WebhookID:   input.WebhookID,
	})
	if err != nil {
		return nil, err
	}

	return &RotateSecretResult{
		RawSecret: result.RawSecret,
		Hint:      result.Hint,
		Version:   result.Config.Version,
	}, nil
}

func (s *Service) ListWebhookDeliveries(ctx context.Context, input ListDeliveriesInput) (*ListDeliveriesResult, error) {
	deliveries, cursor, err := s.listDeliveriesH.Execute(ctx, delivery.ListCommand{
		WorkspaceID: input.WorkspaceID,
		UserID:      input.UserID,
		WebhookID:   input.WebhookID,
		Status:      input.Status,
		EventType:   input.EventType,
		From:        input.From,
		To:          input.To,
		Limit:       input.Limit,
		Cursor:      input.Cursor,
	})
	if err != nil {
		return nil, err
	}

	return &ListDeliveriesResult{Deliveries: deliveries, NextCursor: cursor}, nil
}

func (s *Service) GetWebhookDelivery(ctx context.Context, input GetDeliveryInput) (*domain.WebhookDelivery, error) {
	return s.getDeliveryH.Execute(ctx, delivery.GetCommand{
		WorkspaceID: input.WorkspaceID,
		UserID:      input.UserID,
		DeliveryID:  input.DeliveryID,
	})
}

func (s *Service) RetryWebhookDelivery(ctx context.Context, input RetryDeliveryInput) error {
	return s.retryWebhookDeliveryH.Execute(ctx, delivery.RetryCommand{
		WorkspaceID: input.WorkspaceID,
		UserID:      input.UserID,
		DeliveryID:  input.DeliveryID,
	})
}

func (s *Service) ClaimDueDeliveries(ctx context.Context, limit int, now time.Time) ([]domain.WebhookDelivery, error) {
	return s.deliveryWrite.ClaimPendingDeliveries(ctx, limit, now)
}

func (s *Service) ProcessDueDelivery(ctx context.Context, workspaceID, deliveryID string) (string, error) {
	return s.processDueDeliveryH.Execute(ctx, delivery.ProcessCommand{
		WorkspaceID: workspaceID,
		DeliveryID:  deliveryID,
	})
}

func (s *Service) HandleSourceEvent(ctx context.Context, input HandleSourceEventInput) error {
	return s.handleSourceEventH.Execute(ctx, source.Command{
		RawPayload: input.RawPayload,
	})
}

// SignPayloadRaw signs a webhook payload using HMAC-SHA256.
// It is used by the HTTP deliverer in the infrastructure layer.
func SignPayloadRaw(payload []byte, timestamp, signingKey string) string {
	return delivery.SignPayloadRaw(payload, timestamp, signingKey)
}
