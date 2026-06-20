package mfa

import (
	"context"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/ports"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/transaction"
)

type TOTPDisableOptions struct {
	Totp         ports.TOTPRepository
	TotpVerifier ports.TOTPCodeVerifier
	UsersWrite   ports.UserWriteRepository
	UnitOfWork   ports.UnitOfWork
	Logger       *slog.Logger
}

type TOTPDisableHandler struct {
	totp         ports.TOTPRepository
	totpVerifier ports.TOTPCodeVerifier
	usersWrite   ports.UserWriteRepository
	unitOfWork   ports.UnitOfWork
	log          *slog.Logger
}

type TOTPDisableCommand struct {
	UserID string
	Code   string
	Now    time.Time
}

func NewTOTPDisableHandler(opts TOTPDisableOptions) *TOTPDisableHandler {
	return &TOTPDisableHandler{
		totp:         opts.Totp,
		totpVerifier: opts.TotpVerifier,
		usersWrite:   opts.UsersWrite,
		unitOfWork:   opts.UnitOfWork,
		log:          opts.Logger.With("usecase", "mfa_totp_disable"),
	}
}

func (h *TOTPDisableHandler) Execute(ctx context.Context, cmd TOTPDisableCommand) error {
	if cmd.Code == "" {
		h.log.Warn("MFA disable requires TOTP code", "user_id", cmd.UserID)
		return domain.ErrMFAInvalidCode
	}
	secret, err := h.totp.FindSecretByUser(ctx, cmd.UserID)
	if err != nil {
		h.log.Warn("failed to find TOTP secret for MFA disable", "user_id", cmd.UserID, "error", err)
		return domain.ErrMFAInvalidCode
	}
	if secret == nil {
		h.log.Warn("TOTP secret is nil for MFA disable", "user_id", cmd.UserID)
		return domain.ErrMFAInvalidCode
	}
	if !h.totpVerifier.VerifyTOTPCode(secret.Secret, cmd.Code, cmd.Now) {
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
