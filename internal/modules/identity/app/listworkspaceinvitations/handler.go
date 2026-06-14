package listworkspaceinvitations

import (
	"context"
	"errors"
	"log/slog"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/ports"
)

type Handler struct {
	membershipsRead ports.MembershipReadRepository
	invitationsRead ports.InvitationReadRepository
	log             *slog.Logger
}

func New(membershipsRead ports.MembershipReadRepository, invitationsRead ports.InvitationReadRepository, logger *slog.Logger) *Handler {
	return &Handler{membershipsRead: membershipsRead, invitationsRead: invitationsRead, log: logger.With("usecase", "list_workspace_invitations")}
}

func (h *Handler) Execute(ctx context.Context, workspaceID, userID string) ([]domain.Invitation, error) {
	_, err := h.membershipsRead.FindByWorkspaceAndUser(ctx, workspaceID, userID)
	if err != nil {
		if errors.Is(err, domain.ErrMembershipNotFound) {
			return nil, domain.ErrWorkspaceAccessDenied
		}
		return nil, err
	}
	return h.invitationsRead.ListByWorkspace(ctx, workspaceID)
}
