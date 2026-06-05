package signup

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/usecase"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/id"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/security"
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
	return &Handler{deps: deps, newSession: newSession, log: deps.Logger.With("usecase", "signup")}
}

func (h *Handler) Execute(ctx context.Context, cmd Command) (*usecase.SessionContext, error) {
	email, err := domain.NewEmailAddress(cmd.Email)
	if err != nil {
		return nil, domain.ErrInvalidCredentials
	}
	if cmd.Password == "" {
		return nil, domain.ErrInvalidCredentials
	}
	if err := security.ValidatePasswordPolicy(cmd.Password); err != nil {
		return nil, domain.ErrPasswordPolicy
	}
	existing, err := h.deps.UsersRead.FindByEmail(ctx, email.String())
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		h.log.Error("failed to lookup user by email", "error", err)
		return nil, err
	}
	if existing != nil {
		h.log.Warn("signup attempted with existing email")
		return nil, domain.ErrEmailAlreadyExists
	}
	hash, err := h.deps.Hasher.Hash(cmd.Password)
	if err != nil {
		h.log.Error("failed to hash password", "error", err)
		return nil, err
	}
	user := domain.NewUser(id.Must(id.NewUUIDGenerator()), email, hash, "password", cmd.Now)
	if err := h.deps.UsersWrite.Create(ctx, user); err != nil {
		if isDuplicateError(err) {
			h.log.Warn("signup failed: duplicate email on write", "error", err)
			return nil, domain.ErrEmailAlreadyExists
		}
		h.log.Error("failed to create user", "error", err)
		return nil, err
	}
	h.log.Info("user signed up", "user_id", user.ID)
	if token, err := usecase.CreateEmailVerificationToken(ctx, h.deps, user, cmd.Now); err != nil {
		h.log.Error("failed to create email verification token", "user_id", user.ID, "error", err)
		return nil, err
	} else if token != "" {
		link := usecase.BuildEmailVerificationLink(h.deps, token)
		if err := usecase.SendVerificationEmail(ctx, h.deps, user.Email, link); err != nil {
			h.log.Error("failed to send verification email", "user_id", user.ID, "error", err)
			return nil, err
		}
	}
	return h.newSession(ctx, usecase.NewSessionInput{User: user, Method: "password", IP: cmd.IP, UA: cmd.UA, Now: cmd.Now})
}

func isDuplicateError(err error) bool {
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "duplicate") || strings.Contains(s, "unique")
}
