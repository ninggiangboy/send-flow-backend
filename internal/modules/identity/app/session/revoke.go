package session

import (
	"context"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/ports"
)

type Options struct {
	SessionsWrite ports.SessionWriteRepository
	RefreshStore  ports.RefreshStore
	Logger        *slog.Logger
}

type RevokeSessionHandler struct {
	sessionsWrite ports.SessionWriteRepository
	refreshStore  ports.RefreshStore
	log           *slog.Logger
}

func NewRevokeSessionHandler(opts Options) *RevokeSessionHandler {
	return &RevokeSessionHandler{
		sessionsWrite: opts.SessionsWrite,
		refreshStore:  opts.RefreshStore,
		log:           opts.Logger.With("usecase", "revoke_session"),
	}
}

func (h *RevokeSessionHandler) Execute(ctx context.Context, sessionID, userID string, now time.Time) error {
	sess, err := h.sessionsWrite.FindByID(ctx, sessionID)
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
	if err := h.sessionsWrite.RevokeByID(ctx, sessionID, now); err != nil {
		h.log.Error("failed to revoke session", "session_id", sessionID, "error", err)
		return err
	}
	if err := h.refreshStore.Delete(ctx, sess.RefreshJTI); err != nil {
		h.log.Error("failed to delete refresh token mapping", "session_id", sessionID, "error", err)
		return err
	}
	h.log.Info("session revoked", "session_id", sessionID, "user_id", sess.UserID)
	return nil
}
