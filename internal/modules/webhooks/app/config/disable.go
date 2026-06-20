package config

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

type DisableOptions struct {
	ConfigWrite   ports.ConfigWriteRepository
	TxManager     ports.TransactionManager
	OutboxWriter  ports.OutboxWriter
	AccessChecker ports.WorkspaceAccessChecker
	IDGen         func() (string, error)
	Clock         func() time.Time
	Logger        *slog.Logger
}

type DisableCommand struct {
	WorkspaceID string
	UserID      string
	WebhookID   string
}

type DisableHandler struct {
	configWrite   ports.ConfigWriteRepository
	txManager     ports.TransactionManager
	outboxWriter  ports.OutboxWriter
	accessChecker ports.WorkspaceAccessChecker
	idGen         func() (string, error)
	clock         func() time.Time
	log           *slog.Logger
}

func NewDisable(opts DisableOptions) *DisableHandler {
	return &DisableHandler{
		configWrite:   opts.ConfigWrite,
		txManager:     opts.TxManager,
		outboxWriter:  opts.OutboxWriter,
		accessChecker: opts.AccessChecker,
		idGen:         opts.IDGen,
		clock:         opts.Clock,
		log:           opts.Logger.With("usecase", "disable_webhook_config"),
	}
}

func (h *DisableHandler) Execute(ctx context.Context, cmd DisableCommand) error {
	log := h.log.With("workspace_id", cmd.WorkspaceID, "webhook_id", cmd.WebhookID)

	if err := h.accessChecker.RequirePermission(ctx, cmd.WorkspaceID, cmd.UserID, "webhook.manage"); err != nil {
		return err
	}

	now := h.clock()
	if err := h.txManager.WithinTx(ctx, func(txCtx context.Context) error {
		if err := h.configWrite.Disable(txCtx, cmd.WorkspaceID, cmd.WebhookID, now); err != nil {
			return err
		}
		if h.outboxWriter != nil {
			eventID, err := h.idGen()
			if err != nil {
				return err
			}
			payload, err := json.Marshal(webhookscontracts.ConfigDisabledPayload{
				ConfigID:    cmd.WebhookID,
				WorkspaceID: cmd.WorkspaceID,
				DisabledAt:  now.UTC().Format(time.RFC3339),
			})
			if err != nil {
				return err
			}
			if err := h.outboxWriter.Save(txCtx, ports.OutboxEvent{
				ID:            eventID,
				AggregateType: "webhook_config",
				AggregateID:   cmd.WebhookID,
				EventType:     webhookscontracts.EventConfigDisabledV1,
				Payload:       payload,
				WorkspaceID:   cmd.WorkspaceID,
				OccurredAt:    now,
			}); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		if errors.Is(err, domain.ErrConfigNotFound) {
			return err
		}
		log.Error("failed to disable webhook config", "error", err)
		return err
	}

	log.Info("webhook config disabled")
	return nil
}
