package workspace

import (
	"context"
	"errors"
	"log/slog"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/ports"
)

type GetHandler struct {
	workspacesRead  ports.WorkspaceReadRepository
	membershipsRead ports.MembershipReadRepository
	log             *slog.Logger
}

func NewGetHandler(workspacesRead ports.WorkspaceReadRepository, membershipsRead ports.MembershipReadRepository, logger *slog.Logger) *GetHandler {
	return &GetHandler{workspacesRead: workspacesRead, membershipsRead: membershipsRead, log: logger.With("usecase", "get_workspace")}
}

func (h *GetHandler) Execute(ctx context.Context, workspaceID, userID string) (*domain.Workspace, error) {
	_, err := h.membershipsRead.FindByWorkspaceAndUser(ctx, workspaceID, userID)
	if err != nil {
		if errors.Is(err, domain.ErrMembershipNotFound) {
			return nil, domain.ErrWorkspaceAccessDenied
		}
		return nil, err
	}
	return h.workspacesRead.FindByID(ctx, workspaceID)
}

type ListHandler struct {
	workspacesRead ports.WorkspaceReadRepository
	log            *slog.Logger
}

func NewListHandler(workspacesRead ports.WorkspaceReadRepository, logger *slog.Logger) *ListHandler {
	return &ListHandler{workspacesRead: workspacesRead, log: logger.With("usecase", "list_workspaces")}
}

func (h *ListHandler) Execute(ctx context.Context, userID string) ([]domain.Workspace, error) {
	return h.workspacesRead.ListByUser(ctx, userID)
}
