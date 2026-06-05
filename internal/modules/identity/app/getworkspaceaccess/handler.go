package getworkspaceaccess

import (
	"context"
	"errors"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/usecase"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
)

// Handler returns the requesting user's membership for a workspace, including role and
// permission information. Non-members receive ErrWorkspaceAccessDenied.
type Handler struct{ deps usecase.Deps }

func New(deps usecase.Deps) *Handler { return &Handler{deps: deps} }

// Execute returns the enriched membership (with permissions) for the user in the workspace.
func (h *Handler) Execute(ctx context.Context, workspaceID, userID string) (*domain.Membership, error) {
	membership, err := h.deps.MembershipsRead.FindByWorkspaceAndUser(ctx, workspaceID, userID)
	if err != nil {
		if errors.Is(err, domain.ErrMembershipNotFound) {
			return nil, domain.ErrWorkspaceAccessDenied
		}
		return nil, err
	}
	return membership, nil
}
