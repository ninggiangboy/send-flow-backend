package auth

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/shared"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/ports"
)

type LoginOptions struct {
	UsersWrite      ports.UserWriteRepository
	Hasher          domain.PasswordHasher
	AuthTokens      *shared.AuthTokenService
	SessionFactory  *shared.SessionFactory
	MfaChallengeTTL time.Duration
	Logger          *slog.Logger
}

type LoginHandler struct {
	usersWrite      ports.UserWriteRepository
	hasher          domain.PasswordHasher
	authTokens      *shared.AuthTokenService
	sessionFactory  *shared.SessionFactory
	mfaChallengeTTL time.Duration
	log             *slog.Logger
}

type LoginCommand struct {
	Email    string
	Password string
	IP       string
	UA       string
	Now      time.Time
}

func NewLoginHandler(opts LoginOptions) *LoginHandler {
	return &LoginHandler{
		usersWrite:      opts.UsersWrite,
		hasher:          opts.Hasher,
		authTokens:      opts.AuthTokens,
		sessionFactory:  opts.SessionFactory,
		mfaChallengeTTL: opts.MfaChallengeTTL,
		log:             opts.Logger.With("usecase", "login"),
	}
}

func (h *LoginHandler) Execute(ctx context.Context, cmd LoginCommand) (*shared.LoginResult, error) {
	email, err := domain.NewEmailAddress(cmd.Email)
	if err != nil {
		h.log.Warn("invalid credentials: email parse failed")
		return nil, domain.ErrInvalidCredentials
	}
	user, err := h.usersWrite.FindByEmail(ctx, email.String())
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
		return &shared.LoginResult{User: user, MFARequired: true, MFAChallengeToken: challenge}, nil
	}
	sctx, err := h.sessionFactory.NewSession(ctx, shared.NewSessionInput{User: *user, Method: "password", IP: cmd.IP, UA: cmd.UA, Now: cmd.Now})
	if err != nil {
		return nil, err
	}
	return &shared.LoginResult{SessionContext: sctx, User: user}, nil
}
