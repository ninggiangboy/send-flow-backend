package session

import (
	"context"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/ports"
)

type ListSessionsHandler struct {
	sessionsRead ports.SessionReadRepository
	log          *slog.Logger
}

func NewListSessionsHandler(sessionsRead ports.SessionReadRepository, logger *slog.Logger) *ListSessionsHandler {
	return &ListSessionsHandler{sessionsRead: sessionsRead, log: logger.With("usecase", "list_sessions")}
}

func (h *ListSessionsHandler) Execute(ctx context.Context, userID string, now time.Time) ([]domain.Session, error) {
	return h.sessionsRead.ListByUser(ctx, userID, now)
}
