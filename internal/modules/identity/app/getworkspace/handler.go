package getworkspace

import (
	"context"
	"errors"
	"log/slog"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/ports"
)

type Handler struct {
	workspacesRead  ports.WorkspaceReadRepository
	membershipsRead ports.MembershipReadRepository
	log             *slog.Logger
}

func New(workspacesRead ports.WorkspaceReadRepository, membershipsRead ports.MembershipReadRepository, logger *slog.Logger) *Handler {
	return &Handler{workspacesRead: workspacesRead, membershipsRead: membershipsRead, log: logger.With("usecase", "get_workspace")}
}

func (h *Handler) Execute(ctx context.Context, workspaceID, userID string) (*domain.Workspace, error) {
	_, err := h.membershipsRead.FindByWorkspaceAndUser(ctx, workspaceID, userID)
	if err != nil {
		if errors.Is(err, domain.ErrMembershipNotFound) {
			return nil, domain.ErrWorkspaceAccessDenied
		}
		return nil, err
	}
	return h.workspacesRead.FindByID(ctx, workspaceID)
}
