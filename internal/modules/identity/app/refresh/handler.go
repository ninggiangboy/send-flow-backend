package refresh

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/usecase"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/ports"
)

type Options struct {
	Tokens        ports.TokenManager
	RefreshStore  ports.RefreshStore
	SessionsWrite ports.SessionWriteRepository
	UsersWrite    ports.UserWriteRepository
	Logger        *slog.Logger
}

type Handler struct {
	tokens        ports.TokenManager
	refreshStore  ports.RefreshStore
	sessionsWrite ports.SessionWriteRepository
	usersWrite    ports.UserWriteRepository
	log           *slog.Logger
}

type Command struct {
	RefreshToken string
	Now          time.Time
}

func New(opts Options) *Handler {
	return &Handler{
		tokens:        opts.Tokens,
		refreshStore:  opts.RefreshStore,
		sessionsWrite: opts.SessionsWrite,
		usersWrite:    opts.UsersWrite,
		log:           opts.Logger.With("usecase", "refresh"),
	}
}

func (h *Handler) Execute(ctx context.Context, cmd Command) (*usecase.SessionContext, error) {
	claims, err := h.tokens.ParseRefresh(cmd.RefreshToken, cmd.Now)
	if err != nil {
		h.log.Warn("invalid refresh token: parse failed")
		return nil, domain.ErrUnauthorized
	}
	sessionID, err := h.refreshStore.Find(ctx, claims.JWTID)
	if err != nil {
		if errors.Is(err, ports.ErrCacheMiss) {
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
	sess, err := h.sessionsWrite.FindByID(ctx, claims.SessionID)
	if err != nil || !sess.IsActive(cmd.Now) || sess.RefreshJTI != claims.JWTID {
		h.log.Warn("invalid refresh token: session inactive or JTI mismatch", "session_id", claims.SessionID)
		return nil, domain.ErrUnauthorized
	}
	user, err := h.usersWrite.FindByID(ctx, sess.UserID)
	if err != nil {
		h.log.Warn("invalid refresh token: user not found", "user_id", sess.UserID)
		return nil, domain.ErrUnauthorized
	}
	tokens, accessJTI, refreshJTI, err := h.tokens.Issue(user.ID, sess.ID, cmd.Now)
	if err != nil {
		h.log.Error("failed to issue tokens during refresh", "user_id", user.ID, "session_id", sess.ID, "error", err)
		return nil, err
	}
	if err := h.sessionsWrite.RotateTokens(ctx, sess.ID, accessJTI, refreshJTI, tokens.RefreshExpiresAt, cmd.Now); err != nil {
		h.log.Error("failed to rotate session tokens", "session_id", sess.ID, "error", err)
		return nil, err
	}
	if err := h.refreshStore.Replace(ctx, sess.RefreshJTI, refreshJTI, sess.ID, time.Until(tokens.RefreshExpiresAt)); err != nil {
		h.log.Error("failed to replace refresh token mapping", "session_id", sess.ID, "error", err)
		return nil, err
	}
	sess.AccessJTI = accessJTI
	sess.RefreshJTI = refreshJTI
	sess.ExpiresAt = tokens.RefreshExpiresAt
	h.log.Info("session refreshed", "user_id", user.ID, "session_id", sess.ID)
	return &usecase.SessionContext{Session: *sess, User: *user, Tokens: tokens}, nil
}
