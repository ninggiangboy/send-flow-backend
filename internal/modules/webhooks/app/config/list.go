package config

import (
	"context"
	"log/slog"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/webhooks/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/webhooks/ports"
)

type ListOptions struct {
	ConfigRead    ports.ConfigReadRepository
	AccessChecker ports.WorkspaceAccessChecker
	Logger        *slog.Logger
}

type ListCommand struct {
	WorkspaceID string
	UserID      string
}

type ListHandler struct {
	configRead    ports.ConfigReadRepository
	accessChecker ports.WorkspaceAccessChecker
	log           *slog.Logger
}

func NewList(opts ListOptions) *ListHandler {
	return &ListHandler{
		configRead:    opts.ConfigRead,
		accessChecker: opts.AccessChecker,
		log:           opts.Logger.With("usecase", "list_webhook_configs"),
	}
}

func (h *ListHandler) Execute(ctx context.Context, cmd ListCommand) ([]domain.WebhookConfig, error) {
	log := h.log.With("workspace_id", cmd.WorkspaceID)

	if err := h.accessChecker.RequirePermission(ctx, cmd.WorkspaceID, cmd.UserID, "webhook.manage"); err != nil {
		return nil, err
	}

	configs, err := h.configRead.ListByWorkspace(ctx, cmd.WorkspaceID)
	if err != nil {
		log.Error("failed to list webhook configs", "error", err)
		return nil, err
	}

	return configs, nil
}
