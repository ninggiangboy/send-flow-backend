package listworkspaces

import (
	"context"
	"log/slog"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/ports"
)

type Handler struct {
	workspacesRead ports.WorkspaceReadRepository
	log            *slog.Logger
}

func New(workspacesRead ports.WorkspaceReadRepository, logger *slog.Logger) *Handler {
	return &Handler{workspacesRead: workspacesRead, log: logger.With("usecase", "list_workspaces")}
}

func (h *Handler) Execute(ctx context.Context, userID string) ([]domain.Workspace, error) {
	return h.workspacesRead.ListByUser(ctx, userID)
}
