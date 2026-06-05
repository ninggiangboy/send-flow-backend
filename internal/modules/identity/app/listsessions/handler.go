package listsessions

import (
	"context"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/usecase"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
)

type Handler struct{ deps usecase.Deps }

func New(deps usecase.Deps) *Handler { return &Handler{deps: deps} }

func (h *Handler) Execute(ctx context.Context, userID string, now time.Time) ([]domain.Session, error) {
	return h.deps.SessionsRead.ListByUser(ctx, userID, now)
}
