package listsessions

import (
	"context"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/ports"
)

type Handler struct {
	sessionsRead ports.SessionReadRepository
	log          *slog.Logger
}

func New(sessionsRead ports.SessionReadRepository, logger *slog.Logger) *Handler {
	return &Handler{sessionsRead: sessionsRead, log: logger.With("usecase", "list_sessions")}
}

func (h *Handler) Execute(ctx context.Context, userID string, now time.Time) ([]domain.Session, error) {
	return h.sessionsRead.ListByUser(ctx, userID, now)
}
