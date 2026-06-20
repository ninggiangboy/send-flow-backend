package membership

import (
	"context"
	"errors"
	"log/slog"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/ports"
)

type ListMembersHandler struct {
	membershipsRead ports.MembershipReadRepository
	log             *slog.Logger
}

func NewListMembersHandler(membershipsRead ports.MembershipReadRepository, logger *slog.Logger) *ListMembersHandler {
	return &ListMembersHandler{membershipsRead: membershipsRead, log: logger.With("usecase", "list_workspace_members")}
}

func (h *ListMembersHandler) Execute(ctx context.Context, workspaceID, userID string) ([]domain.Membership, error) {
	_, err := h.membershipsRead.FindByWorkspaceAndUser(ctx, workspaceID, userID)
	if err != nil {
		if errors.Is(err, domain.ErrMembershipNotFound) {
			return nil, domain.ErrWorkspaceAccessDenied
		}
		return nil, err
	}
	return h.membershipsRead.ListByWorkspace(ctx, workspaceID)
}

type GetAccessHandler struct {
	membershipsRead ports.MembershipReadRepository
	log             *slog.Logger
}

func NewGetAccessHandler(membershipsRead ports.MembershipReadRepository, logger *slog.Logger) *GetAccessHandler {
	return &GetAccessHandler{membershipsRead: membershipsRead, log: logger.With("usecase", "get_workspace_access")}
}

func (h *GetAccessHandler) Execute(ctx context.Context, workspaceID, userID string) (*domain.Membership, error) {
	membership, err := h.membershipsRead.FindByWorkspaceAndUser(ctx, workspaceID, userID)
	if err != nil {
		if errors.Is(err, domain.ErrMembershipNotFound) {
			return nil, domain.ErrWorkspaceAccessDenied
		}
		return nil, err
	}
	return membership, nil
}

type ListInvitationsHandler struct {
	membershipsRead ports.MembershipReadRepository
	invitationsRead ports.InvitationReadRepository
	log             *slog.Logger
}

func NewListInvitationsHandler(membershipsRead ports.MembershipReadRepository, invitationsRead ports.InvitationReadRepository, logger *slog.Logger) *ListInvitationsHandler {
	return &ListInvitationsHandler{membershipsRead: membershipsRead, invitationsRead: invitationsRead, log: logger.With("usecase", "list_workspace_invitations")}
}

func (h *ListInvitationsHandler) Execute(ctx context.Context, workspaceID, userID string) ([]domain.Invitation, error) {
	_, err := h.membershipsRead.FindByWorkspaceAndUser(ctx, workspaceID, userID)
	if err != nil {
		if errors.Is(err, domain.ErrMembershipNotFound) {
			return nil, domain.ErrWorkspaceAccessDenied
		}
		return nil, err
	}
	return h.invitationsRead.ListByWorkspace(ctx, workspaceID)
}
