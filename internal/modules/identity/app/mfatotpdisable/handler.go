package mfatotpdisable

import (
	"context"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/ports"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/transaction"
)

type Options struct {
	UsersRead    ports.UserReadRepository
	Hasher       domain.PasswordHasher
	Totp         ports.TOTPRepository
	TotpVerifier ports.TOTPCodeVerifier
	UsersWrite   ports.UserWriteRepository
	UnitOfWork   ports.UnitOfWork
	Logger       *slog.Logger
}

type Handler struct {
	usersRead    ports.UserReadRepository
	hasher       domain.PasswordHasher
	totp         ports.TOTPRepository
	totpVerifier ports.TOTPCodeVerifier
	usersWrite   ports.UserWriteRepository
	unitOfWork   ports.UnitOfWork
	log          *slog.Logger
}

type Command struct {
	UserID   string
	Password string
	Code     string
	Now      time.Time
}

func New(opts Options) *Handler {
	return &Handler{
		usersRead:    opts.UsersRead,
		hasher:       opts.Hasher,
		totp:         opts.Totp,
		totpVerifier: opts.TotpVerifier,
		usersWrite:   opts.UsersWrite,
		unitOfWork:   opts.UnitOfWork,
		log:          opts.Logger.With("usecase", "mfa_totp_disable"),
	}
}

func (h *Handler) Execute(ctx context.Context, cmd Command) error {
	user, err := h.usersRead.FindByID(ctx, cmd.UserID)
	if err != nil {
		h.log.Error("failed to find user for MFA disable", "user_id", cmd.UserID, "error", err)
		return err
	}
	authorized := false
	if cmd.Password != "" && user.HashedPassword != "" && user.VerifyPassword(cmd.Password, h.hasher) == nil {
		authorized = true
	}
	if !authorized && cmd.Code != "" {
		secret, err := h.totp.FindSecretByUser(ctx, cmd.UserID)
		if err == nil && h.totpVerifier.VerifyTOTPCode(secret.Secret, cmd.Code, cmd.Now) {
			authorized = true
		}
	}
	if !authorized {
		h.log.Warn("MFA disable authorization failed", "user_id", cmd.UserID)
		return domain.ErrMFAInvalidCode
	}
	doDisable := func(txCtx context.Context) error {
		if err := h.usersWrite.SetMFAEnabledAt(txCtx, cmd.UserID, nil, cmd.Now); err != nil {
			return err
		}
		if err := h.totp.DeleteSecret(txCtx, cmd.UserID); err != nil {
			return err
		}
		return h.totp.ReplaceRecoveryCodes(txCtx, cmd.UserID, nil)
	}
	if err := transaction.RunInTx(ctx, h.unitOfWork, doDisable); err != nil {
		h.log.Error("failed to disable MFA", "user_id", cmd.UserID, "error", err)
		return err
	}
	h.log.Info("MFA disabled", "user_id", cmd.UserID)
	return nil
}
