package revokesession

import (
	"context"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/usecase"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
)

type Handler struct {
	deps usecase.Deps
	log  *slog.Logger
}

func New(deps usecase.Deps) *Handler {
	return &Handler{deps: deps, log: deps.Logger.With("usecase", "revoke_session")}
}

func (h *Handler) Execute(ctx context.Context, sessionID, userID string, now time.Time) error {
	sess, err := h.deps.SessionsRead.FindByID(ctx, sessionID)
	if err != nil {
		h.log.Error("failed to find session for revocation", "session_id", sessionID, "error", err)
		return err
	}
	if sess.UserID != userID {
		return domain.ErrSessionNotOwned
	}
	if sess.IsRevoked() {
		h.log.Info("session already revoked", "session_id", sessionID)
		return nil
	}
	if err := h.deps.SessionsWrite.RevokeByID(ctx, sessionID, now); err != nil {
		h.log.Error("failed to revoke session", "session_id", sessionID, "error", err)
		return err
	}
	if err := h.deps.RefreshStore.Delete(ctx, sess.RefreshJTI); err != nil {
		h.log.Error("failed to delete refresh token mapping", "session_id", sessionID, "error", err)
		return err
	}
	h.log.Info("session revoked", "session_id", sessionID, "user_id", sess.UserID)
	return nil
}
