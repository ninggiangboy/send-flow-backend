package listworkspacemembers

import (
	"context"
	"errors"
	"log/slog"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/ports"
)

type Handler struct {
	membershipsRead ports.MembershipReadRepository
	log             *slog.Logger
}

func New(membershipsRead ports.MembershipReadRepository, logger *slog.Logger) *Handler {
	return &Handler{membershipsRead: membershipsRead, log: logger.With("usecase", "list_workspace_members")}
}

func (h *Handler) Execute(ctx context.Context, workspaceID, userID string) ([]domain.Membership, error) {
	_, err := h.membershipsRead.FindByWorkspaceAndUser(ctx, workspaceID, userID)
	if err != nil {
		if errors.Is(err, domain.ErrMembershipNotFound) {
			return nil, domain.ErrWorkspaceAccessDenied
		}
		return nil, err
	}
	return h.membershipsRead.ListByWorkspace(ctx, workspaceID)
}
