package verification

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/shared"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/ports"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/transaction"
)

type ResetPasswordOptions struct {
	PasswordValidator ports.PasswordValidator
	AuthTokens        *shared.AuthTokenService
	Hasher            domain.PasswordHasher
	UsersWrite        ports.UserWriteRepository
	SessionsWrite     ports.SessionWriteRepository
	AuthTokensRepo    ports.AuthTokenRepository
	UnitOfWork        ports.UnitOfWork
	Logger            *slog.Logger
}

type ResetPasswordHandler struct {
	passwordValidator ports.PasswordValidator
	authTokens        *shared.AuthTokenService
	hasher            domain.PasswordHasher
	usersWrite        ports.UserWriteRepository
	sessionsWrite     ports.SessionWriteRepository
	authTokensRepo    ports.AuthTokenRepository
	unitOfWork        ports.UnitOfWork
	log               *slog.Logger
}

type ResetPasswordCommand struct {
	Token       string
	NewPassword string
	Now         time.Time
}

func NewResetPasswordHandler(opts ResetPasswordOptions) *ResetPasswordHandler {
	return &ResetPasswordHandler{
		passwordValidator: opts.PasswordValidator,
		authTokens:        opts.AuthTokens,
		hasher:            opts.Hasher,
		usersWrite:        opts.UsersWrite,
		sessionsWrite:     opts.SessionsWrite,
		authTokensRepo:    opts.AuthTokensRepo,
		unitOfWork:        opts.UnitOfWork,
		log:               opts.Logger.With("usecase", "reset_password"),
	}
}

func (h *ResetPasswordHandler) Execute(ctx context.Context, cmd ResetPasswordCommand) error {
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
	doReset := func(txCtx context.Context) error {
		if err := h.usersWrite.UpdatePassword(txCtx, record.UserID, hash, cmd.Now); err != nil {
			h.log.Error("failed to update password", "user_id", record.UserID, "error", err)
			return err
		}
		if err := h.sessionsWrite.RevokeByUser(txCtx, record.UserID, cmd.Now); err != nil {
			h.log.Error("failed to revoke sessions after password reset", "user_id", record.UserID, "error", err)
			return err
		}
		if err := h.authTokensRepo.DeleteByUserAndPurpose(txCtx, record.UserID, domain.AuthTokenPurposeMFAChallenge); err != nil {
			h.log.Error("failed to clear MFA challenges after password reset", "user_id", record.UserID, "error", err)
			return err
		}
		return nil
	}
	if err := transaction.RunInTx(ctx, h.unitOfWork, doReset); err != nil {
		return err
	}
	h.log.Info("password reset completed, all sessions revoked", "user_id", record.UserID)
	return nil
}
