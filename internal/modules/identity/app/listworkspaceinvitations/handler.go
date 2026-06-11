package listworkspaceinvitations

import (
	"context"
	"errors"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/usecase"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
)

type Handler struct{ deps usecase.Deps }

func New(deps usecase.Deps) *Handler { return &Handler{deps: deps} }

func (h *Handler) Execute(ctx context.Context, workspaceID, userID string) ([]domain.Invitation, error) {
	_, err := h.deps.MembershipsRead.FindByWorkspaceAndUser(ctx, workspaceID, userID)
	if err != nil {
		if errors.Is(err, domain.ErrMembershipNotFound) {
			return nil, domain.ErrWorkspaceAccessDenied
		}
		return nil, err
	}
	return h.deps.InvitationsRead.ListByWorkspace(ctx, workspaceID)
}
