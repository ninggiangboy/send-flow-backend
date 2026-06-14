package listwebhookconfigs

import (
	"context"
	"log/slog"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/webhooks/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/webhooks/ports"
)

type Options struct {
	ConfigRead    ports.ConfigReadRepository
	AccessChecker ports.WorkspaceAccessChecker
	Logger        *slog.Logger
}

type Command struct {
	WorkspaceID string
	UserID      string
}

type Handler struct {
	configRead    ports.ConfigReadRepository
	accessChecker ports.WorkspaceAccessChecker
	log           *slog.Logger
}

func New(opts Options) *Handler {
	return &Handler{
		configRead:    opts.ConfigRead,
		accessChecker: opts.AccessChecker,
		log:           opts.Logger.With("usecase", "list_webhook_configs"),
	}
}

func (h *Handler) Execute(ctx context.Context, cmd Command) ([]domain.WebhookConfig, error) {
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
