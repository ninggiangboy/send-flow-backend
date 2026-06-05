package listworkspacemembers

import (
	"context"
	"errors"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/usecase"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
)

// Handler lists all members of a workspace, but only if the requesting user is also a member.
// Non-members receive ErrWorkspaceAccessDenied.
type Handler struct{ deps usecase.Deps }

func New(deps usecase.Deps) *Handler { return &Handler{deps: deps} }

// Execute returns all memberships for the workspace if the user is a member.
func (h *Handler) Execute(ctx context.Context, workspaceID, userID string) ([]domain.Membership, error) {
	_, err := h.deps.MembershipsRead.FindByWorkspaceAndUser(ctx, workspaceID, userID)
	if err != nil {
		if errors.Is(err, domain.ErrMembershipNotFound) {
			return nil, domain.ErrWorkspaceAccessDenied
		}
		return nil, err
	}
	return h.deps.MembershipsRead.ListByWorkspace(ctx, workspaceID)
}
