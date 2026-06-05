package login

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/usecase"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
)

type Handler struct {
	deps       usecase.Deps
	newSession usecase.NewSession
	log        *slog.Logger
}

type Command struct {
	Email    string
	Password string
	IP       string
	UA       string
	Now      time.Time
}

func New(deps usecase.Deps, newSession usecase.NewSession) *Handler {
	return &Handler{deps: deps, newSession: newSession, log: deps.Logger.With("usecase", "login")}
}

func (h *Handler) Execute(ctx context.Context, cmd Command) (*usecase.LoginResult, error) {
	email, err := domain.NewEmailAddress(cmd.Email)
	if err != nil {
		h.log.Warn("invalid credentials: email parse failed")
		return nil, domain.ErrInvalidCredentials
	}
	user, err := h.deps.UsersRead.FindByEmail(ctx, email.String())
	if err != nil {
		if !errors.Is(err, domain.ErrNotFound) {
			h.log.Error("failed to lookup user by email", "error", err)
		}
		h.log.Warn("invalid credentials: user not found")
		return nil, domain.ErrInvalidCredentials
	}
	if err := user.VerifyPassword(cmd.Password, h.deps.Hasher); err != nil {
		h.log.Warn("invalid credentials: password mismatch", "user_id", user.ID)
		return nil, domain.ErrInvalidCredentials
	}
	if user.MFAEnabled() {
		h.log.Info("MFA required for login", "user_id", user.ID)
		challenge, err := usecase.CreateMFAChallenge(ctx, h.deps, user.ID, cmd.Now)
		if err != nil {
			h.log.Error("failed to create MFA challenge", "user_id", user.ID, "error", err)
			return nil, err
		}
		return &usecase.LoginResult{User: user, MFARequired: true, MFAChallengeToken: challenge}, nil
	}
	sctx, err := h.newSession(ctx, usecase.NewSessionInput{User: *user, Method: "password", IP: cmd.IP, UA: cmd.UA, Now: cmd.Now})
	if err != nil {
		return nil, err
	}
	return &usecase.LoginResult{SessionContext: sctx, User: user}, nil
}
