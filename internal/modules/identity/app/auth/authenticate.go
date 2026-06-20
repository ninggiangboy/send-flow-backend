package auth

import (
	"context"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/ports"
)

type AuthenticateHandler struct {
	tokens       ports.TokenManager
	sessionsRead ports.SessionReadRepository
	log          *slog.Logger
}

func NewAuthenticateHandler(tokens ports.TokenManager, sessionsRead ports.SessionReadRepository, logger *slog.Logger) *AuthenticateHandler {
	return &AuthenticateHandler{tokens: tokens, sessionsRead: sessionsRead, log: logger.With("usecase", "authenticate")}
}

func (h *AuthenticateHandler) Execute(ctx context.Context, token string, now time.Time) (*domain.Session, *domain.User, error) {
	claims, err := h.tokens.ParseAccess(token, now)
	if err != nil {
		return nil, nil, domain.ErrUnauthorized
	}
	if claims.SessionID == "" || claims.Subject == "" {
		return nil, nil, domain.ErrUnauthorized
	}
	sess, err := h.sessionsRead.FindByID(ctx, claims.SessionID)
	if err != nil {
		h.log.Warn("session not found during authentication", "session_id", claims.SessionID, "error", err)
		return nil, nil, domain.ErrUnauthorized
	}
	if !sess.IsActive(now) {
		h.log.Warn("session is not active during authentication", "session_id", claims.SessionID)
		return nil, nil, domain.ErrUnauthorized
	}
	return sess, &domain.User{ID: claims.Subject}, nil
}
