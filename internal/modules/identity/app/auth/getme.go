package auth

import (
	"context"
	"log/slog"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/ports"
)

type GetMeHandler struct {
	usersRead ports.UserReadRepository
	log       *slog.Logger
}

func NewGetMeHandler(usersRead ports.UserReadRepository, logger *slog.Logger) *GetMeHandler {
	return &GetMeHandler{usersRead: usersRead, log: logger.With("usecase", "get_me")}
}

func (h *GetMeHandler) Execute(ctx context.Context, userID string) (*domain.User, error) {
	return h.usersRead.FindByID(ctx, userID)
}
