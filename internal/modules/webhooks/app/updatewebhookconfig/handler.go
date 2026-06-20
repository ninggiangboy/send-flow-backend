package updatewebhookconfig

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

type Options struct {
	ConfigWrite   ports.ConfigWriteRepository
	TxManager     ports.TransactionManager
	OutboxWriter  ports.OutboxWriter
	AccessChecker ports.WorkspaceAccessChecker
	IDGen         func() (string, error)
	Clock         func() time.Time
	Logger        *slog.Logger
}

type Command struct {
	WorkspaceID   string
	UserID        string
	WebhookID     string
	Name          *string
	TargetURL     *string
	Subscriptions []string
	Status        *string
}

type Handler struct {
	configWrite   ports.ConfigWriteRepository
	txManager     ports.TransactionManager
	outboxWriter  ports.OutboxWriter
	accessChecker ports.WorkspaceAccessChecker
	idGen         func() (string, error)
	clock         func() time.Time
	log           *slog.Logger
}

func New(opts Options) *Handler {
	return &Handler{
		configWrite:   opts.ConfigWrite,
		txManager:     opts.TxManager,
		outboxWriter:  opts.OutboxWriter,
		accessChecker: opts.AccessChecker,
		idGen:         opts.IDGen,
		clock:         opts.Clock,
		log:           opts.Logger.With("usecase", "update_webhook_config"),
	}
}

func (h *Handler) Execute(ctx context.Context, cmd Command) (*domain.WebhookConfig, error) {
	log := h.log.With("workspace_id", cmd.WorkspaceID, "webhook_id", cmd.WebhookID)

	if err := h.accessChecker.RequirePermission(ctx, cmd.WorkspaceID, cmd.UserID, "webhook.manage"); err != nil {
		return nil, err
	}

	cfg, err := h.configWrite.FindByID(ctx, cmd.WorkspaceID, cmd.WebhookID)
	if err != nil {
		if errors.Is(err, domain.ErrConfigNotFound) {
			return nil, err
		}
		log.Error("failed to find webhook config", "error", err)
		return nil, err
	}

	if cmd.Name != nil {
		cfg.Name = *cmd.Name
	}
	if cmd.TargetURL != nil {
		cfg.TargetURL = *cmd.TargetURL
	}
	if cmd.Subscriptions != nil {
		cfg.Subscriptions = cmd.Subscriptions
	}
	if cmd.Status != nil {
		cfg.Status = domain.ConfigStatus(*cmd.Status)
	}

	if err := domain.ValidateWebhookConfig(*cfg); err != nil {
		log.Warn("invalid webhook config update", "error", err)
		return nil, err
	}

	cfg.Version++
	cfg.UpdatedAt = h.clock()

	if err := h.txManager.WithinTx(ctx, func(txCtx context.Context) error {
		if err := h.configWrite.Update(txCtx, *cfg); err != nil {
			return err
		}
		if h.outboxWriter != nil {
			eventID, err := h.idGen()
			if err != nil {
				return err
			}
			payload, err := json.Marshal(webhookscontracts.ConfigUpdatedPayload{
				ConfigID:    cfg.ID,
				WorkspaceID: cfg.WorkspaceID,
				Version:     cfg.Version,
				Status:      string(cfg.Status),
				Subscribed:  cfg.Subscriptions,
			})
			if err != nil {
				return err
			}
			if err := h.outboxWriter.Save(txCtx, ports.OutboxEvent{
				ID:            eventID,
				AggregateType: "webhook_config",
				AggregateID:   cfg.ID,
				EventType:     webhookscontracts.EventConfigUpdatedV1,
				Payload:       payload,
				WorkspaceID:   cfg.WorkspaceID,
				OccurredAt:    cfg.UpdatedAt,
			}); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		if errors.Is(err, domain.ErrConfigNameConflict) {
			return nil, err
		}
		if errors.Is(err, domain.ErrRotateConflict) {
			return nil, domain.ErrRotateConflict
		}
		log.Error("failed to update webhook config", "error", err)
		return nil, err
	}

	log.Info("webhook config updated", "version", cfg.Version)
	return cfg, nil
}
