package authenticate

import (
	"context"
	"log/slog"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/ports"
)

type Handler struct {
	tokens ports.TokenManager
	log    *slog.Logger
}

func New(tokens ports.TokenManager, logger *slog.Logger) *Handler {
	return &Handler{tokens: tokens, log: logger.With("usecase", "authenticate")}
}

func (h *Handler) Execute(ctx context.Context, token string) (*domain.Session, *domain.User, error) {
	claims, err := h.tokens.ParseAccess(token)
	if err != nil {
		return nil, nil, domain.ErrUnauthorized
	}
	if claims.SessionID == "" || claims.Subject == "" {
		return nil, nil, domain.ErrUnauthorized
	}
	return &domain.Session{ID: claims.SessionID, UserID: claims.Subject, AccessJTI: claims.JWTID}, &domain.User{ID: claims.Subject}, nil
}
