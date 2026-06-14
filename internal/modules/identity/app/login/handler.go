package login

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
	UsersRead       ports.UserReadRepository
	Hasher          domain.PasswordHasher
	AuthTokens      *usecase.AuthTokenService
	SessionFactory  *usecase.SessionFactory
	MfaChallengeTTL time.Duration
	Logger          *slog.Logger
}

type Handler struct {
	usersRead       ports.UserReadRepository
	hasher          domain.PasswordHasher
	authTokens      *usecase.AuthTokenService
	sessionFactory  *usecase.SessionFactory
	mfaChallengeTTL time.Duration
	log             *slog.Logger
}

type Command struct {
	Email    string
	Password string
	IP       string
	UA       string
	Now      time.Time
}

func New(opts Options) *Handler {
	return &Handler{
		usersRead:       opts.UsersRead,
		hasher:          opts.Hasher,
		authTokens:      opts.AuthTokens,
		sessionFactory:  opts.SessionFactory,
		mfaChallengeTTL: opts.MfaChallengeTTL,
		log:             opts.Logger.With("usecase", "login"),
	}
}

func (h *Handler) Execute(ctx context.Context, cmd Command) (*usecase.LoginResult, error) {
	email, err := domain.NewEmailAddress(cmd.Email)
	if err != nil {
		h.log.Warn("invalid credentials: email parse failed")
		return nil, domain.ErrInvalidCredentials
	}
	user, err := h.usersRead.FindByEmail(ctx, email.String())
	if err != nil {
		if !errors.Is(err, domain.ErrNotFound) {
			h.log.Error("failed to lookup user by email", "error", err)
		}
		h.log.Warn("invalid credentials", "user_id", cmd.Email)
		return nil, domain.ErrInvalidCredentials
	}
	if err := user.VerifyPassword(cmd.Password, h.hasher); err != nil {
		h.log.Warn("invalid credentials", "user_id", user.ID)
		return nil, domain.ErrInvalidCredentials
	}
	if user.MFAEnabled() {
		h.log.Info("MFA required for login", "user_id", user.ID)
		challenge, err := h.authTokens.CreateToken(ctx, user.ID, domain.AuthTokenPurposeMFAChallenge, h.mfaChallengeTTL, cmd.Now)
		if err != nil {
			h.log.Error("failed to create MFA challenge", "user_id", user.ID, "error", err)
			return nil, err
		}
		return &usecase.LoginResult{User: user, MFARequired: true, MFAChallengeToken: challenge}, nil
	}
	sctx, err := h.sessionFactory.NewSession(ctx, usecase.NewSessionInput{User: *user, Method: "password", IP: cmd.IP, UA: cmd.UA, Now: cmd.Now})
	if err != nil {
		return nil, err
	}
	return &usecase.LoginResult{SessionContext: sctx, User: user}, nil
}
