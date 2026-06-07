package refresh

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/usecase"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
	redispkg "github.com/ninggiangboy/send-flow/backend/internal/platform/redis"
)

type Handler struct {
	deps usecase.Deps
	log  *slog.Logger
}

type Command struct {
	RefreshToken string
	Now          time.Time
}

func New(deps usecase.Deps) *Handler {
	return &Handler{deps: deps, log: deps.Logger.With("usecase", "refresh")}
}

func (h *Handler) Execute(ctx context.Context, cmd Command) (*usecase.SessionContext, error) {
	claims, err := h.deps.Tokens.ParseRefresh(cmd.RefreshToken)
	if err != nil {
		h.log.Warn("invalid refresh token: parse failed")
		return nil, domain.ErrUnauthorized
	}
	sessionID, err := h.deps.RefreshStore.Find(ctx, claims.JWTID)
	if err != nil {
		if errors.Is(err, redispkg.ErrCacheMiss) {
			h.log.Warn("invalid refresh token: JTI not found or already rotated", "session_id", claims.SessionID)
			return nil, domain.ErrUnauthorized
		}
		h.log.Error("failed to look up refresh token JTI", "error", err)
		return nil, err
	}
	if sessionID != claims.SessionID {
		h.log.Warn("invalid refresh token: session ID mismatch", "session_id", claims.SessionID)
		return nil, domain.ErrUnauthorized
	}
	sess, err := h.deps.SessionsRead.FindByID(ctx, claims.SessionID)
	if err != nil || !sess.IsActive(cmd.Now) || sess.RefreshJTI != claims.JWTID {
		h.log.Warn("invalid refresh token: session inactive or JTI mismatch", "session_id", claims.SessionID)
		return nil, domain.ErrUnauthorized
	}
	user, err := h.deps.UsersRead.FindByID(ctx, sess.UserID)
	if err != nil {
		h.log.Warn("invalid refresh token: user not found", "user_id", sess.UserID)
		return nil, domain.ErrUnauthorized
	}
	tokens, accessJTI, refreshJTI, err := h.deps.Tokens.Issue(user.ID, sess.ID, cmd.Now)
	if err != nil {
		h.log.Error("failed to issue tokens during refresh", "user_id", user.ID, "session_id", sess.ID, "error", err)
		return nil, err
	}
	if err := h.deps.SessionsWrite.RotateTokens(ctx, sess.ID, accessJTI, refreshJTI, tokens.RefreshExpiresAt, cmd.Now); err != nil {
		h.log.Error("failed to rotate session tokens", "session_id", sess.ID, "error", err)
		return nil, err
	}
	if err := h.deps.RefreshStore.Replace(ctx, sess.RefreshJTI, refreshJTI, sess.ID, time.Until(tokens.RefreshExpiresAt)); err != nil {
		h.log.Error("failed to replace refresh token mapping", "session_id", sess.ID, "error", err)
		return nil, err
	}
	sess.AccessJTI = accessJTI
	sess.RefreshJTI = refreshJTI
	sess.ExpiresAt = tokens.RefreshExpiresAt
	h.log.Info("session refreshed", "user_id", user.ID, "session_id", sess.ID)
	return &usecase.SessionContext{Session: *sess, User: *user, Tokens: tokens}, nil
}
