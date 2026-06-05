package getworkspace

import (
	"context"
	"errors"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/usecase"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
)

// Handler fetches a workspace by ID, but only if the requesting user is a member.
// Non-members receive ErrWorkspaceAccessDenied regardless of whether the workspace exists,
// preventing workspace ID enumeration.
type Handler struct{ deps usecase.Deps }

func New(deps usecase.Deps) *Handler { return &Handler{deps: deps} }

// Execute returns the workspace if the user is a member; otherwise ErrWorkspaceAccessDenied.
func (h *Handler) Execute(ctx context.Context, workspaceID, userID string) (*domain.Workspace, error) {
	_, err := h.deps.MembershipsRead.FindByWorkspaceAndUser(ctx, workspaceID, userID)
	if err != nil {
		if errors.Is(err, domain.ErrMembershipNotFound) {
			return nil, domain.ErrWorkspaceAccessDenied
		}
		return nil, err
	}
	return h.deps.WorkspacesRead.FindByID(ctx, workspaceID)
}
