package listworkspaces

import (
	"context"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/usecase"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
)

type Handler struct{ deps usecase.Deps }

func New(deps usecase.Deps) *Handler { return &Handler{deps: deps} }

func (h *Handler) Execute(ctx context.Context, userID string) ([]domain.Workspace, error) {
	return h.deps.WorkspacesRead.ListByUser(ctx, userID)
}
