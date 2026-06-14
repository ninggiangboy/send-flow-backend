package getme

import (
	"context"
	"log/slog"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/ports"
)

type Handler struct {
	usersRead ports.UserReadRepository
	log       *slog.Logger
}

func New(usersRead ports.UserReadRepository, logger *slog.Logger) *Handler {
	return &Handler{usersRead: usersRead, log: logger.With("usecase", "get_me")}
}

func (h *Handler) Execute(ctx context.Context, userID string) (*domain.User, error) {
	return h.usersRead.FindByID(ctx, userID)
}
