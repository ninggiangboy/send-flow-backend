package resetpassword

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
	PasswordValidator ports.PasswordValidator
	AuthTokens        *usecase.AuthTokenService
	Hasher            domain.PasswordHasher
	UsersWrite        ports.UserWriteRepository
	SessionsWrite     ports.SessionWriteRepository
	AuthTokensRepo    ports.AuthTokenRepository
	Logger            *slog.Logger
}

type Handler struct {
	passwordValidator ports.PasswordValidator
	authTokens        *usecase.AuthTokenService
	hasher            domain.PasswordHasher
	usersWrite        ports.UserWriteRepository
	sessionsWrite     ports.SessionWriteRepository
	authTokensRepo    ports.AuthTokenRepository
	log               *slog.Logger
}

type Command struct {
	Token       string
	NewPassword string
	Now         time.Time
}

func New(opts Options) *Handler {
	return &Handler{
		passwordValidator: opts.PasswordValidator,
		authTokens:        opts.AuthTokens,
		hasher:            opts.Hasher,
		usersWrite:        opts.UsersWrite,
		sessionsWrite:     opts.SessionsWrite,
		authTokensRepo:    opts.AuthTokensRepo,
		log:               opts.Logger.With("usecase", "reset_password"),
	}
}

func (h *Handler) Execute(ctx context.Context, cmd Command) error {
	if err := h.passwordValidator.Validate(cmd.NewPassword); err != nil {
		return domain.ErrPasswordPolicy
	}
	record, err := h.authTokens.ConsumeToken(ctx, cmd.Token, domain.AuthTokenPurposePasswordReset, cmd.Now)
	if err != nil {
		if errors.Is(err, domain.ErrUnauthorized) || errors.Is(err, domain.ErrNotFound) {
			h.log.Warn("invalid or expired password reset token")
			return domain.ErrResetToken
		}
		h.log.Error("failed to consume password reset token", "error", err)
		return err
	}
	hash, err := h.hasher.Hash(cmd.NewPassword)
	if err != nil {
		h.log.Error("failed to hash new password", "user_id", record.UserID, "error", err)
		return err
	}
	if err := h.usersWrite.UpdatePassword(ctx, record.UserID, hash, cmd.Now); err != nil {
		h.log.Error("failed to update password", "user_id", record.UserID, "error", err)
		return err
	}
	if err := h.sessionsWrite.RevokeByUser(ctx, record.UserID, cmd.Now); err != nil {
		h.log.Error("failed to revoke sessions after password reset", "user_id", record.UserID, "error", err)
		return err
	}
	if err := h.authTokensRepo.DeleteByUserAndPurpose(ctx, record.UserID, domain.AuthTokenPurposeMFAChallenge); err != nil {
		h.log.Error("failed to clear MFA challenges after password reset", "user_id", record.UserID, "error", err)
		return err
	}
	h.log.Info("password reset completed, all sessions revoked", "user_id", record.UserID)
	return nil
}
