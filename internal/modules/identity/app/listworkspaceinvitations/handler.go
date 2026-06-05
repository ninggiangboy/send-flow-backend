package listworkspaceinvitations

import (
	"context"
	"errors"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/usecase"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
)

// Handler lists all invitations for a workspace. Only owners and admins may view invitations;
// other members receive ErrMembershipManageDenied.
//
// Note: access control here uses the legacy membership role (Role field) rather than the
// fine-grained permission system, matching the current API contract.
type Handler struct{ deps usecase.Deps }

func New(deps usecase.Deps) *Handler { return &Handler{deps: deps} }

// Execute returns all invitations for the workspace if the user is an owner or admin.
func (h *Handler) Execute(ctx context.Context, workspaceID, userID string) ([]domain.Invitation, error) {
	membership, err := h.deps.MembershipsRead.FindByWorkspaceAndUser(ctx, workspaceID, userID)
	if err != nil {
		if errors.Is(err, domain.ErrMembershipNotFound) {
			return nil, domain.ErrWorkspaceAccessDenied
		}
		return nil, err
	}
	if membership.Role != domain.MembershipRoleOwner && membership.Role != domain.MembershipRoleAdmin {
		return nil, domain.ErrMembershipManageDenied
	}
	return h.deps.InvitationsRead.ListByWorkspace(ctx, workspaceID)
}
