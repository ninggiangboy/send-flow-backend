package mfatotpenable

import (
	"context"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/ports"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/transaction"
)

type Options struct {
	Totp            ports.TOTPRepository
	TotpVerifier    ports.TOTPCodeVerifier
	RecoveryCodeGen ports.RecoveryCodeGenerator
	IdGen           ports.IDGenerator
	TokenHasher     ports.TokenHasher
	UsersWrite      ports.UserWriteRepository
	UnitOfWork      ports.UnitOfWork
	Logger          *slog.Logger
}

type Handler struct {
	totp            ports.TOTPRepository
	totpVerifier    ports.TOTPCodeVerifier
	recoveryCodeGen ports.RecoveryCodeGenerator
	idGen           ports.IDGenerator
	tokenHasher     ports.TokenHasher
	usersWrite      ports.UserWriteRepository
	unitOfWork      ports.UnitOfWork
	log             *slog.Logger
}

type Result struct {
	RecoveryCodes []string `json:"recovery_codes"`
}

func New(opts Options) *Handler {
	return &Handler{
		totp:            opts.Totp,
		totpVerifier:    opts.TotpVerifier,
		recoveryCodeGen: opts.RecoveryCodeGen,
		idGen:           opts.IdGen,
		tokenHasher:     opts.TokenHasher,
		usersWrite:      opts.UsersWrite,
		unitOfWork:      opts.UnitOfWork,
		log:             opts.Logger.With("usecase", "mfa_totp_enable"),
	}
}

func (h *Handler) Execute(ctx context.Context, userID, code string, now time.Time) (*Result, error) {
	secret, err := h.totp.FindSecretByUser(ctx, userID)
	if err != nil || !h.totpVerifier.VerifyTOTPCode(secret.Secret, code, now) {
		h.log.Warn("invalid TOTP code during MFA enable", "user_id", userID)
		return nil, domain.ErrMFAInvalidCode
	}
	recoveryCodes := make([]string, 0, 8)
	records := make([]domain.RecoveryCode, 0, 8)
	for range 8 {
		raw, err := h.recoveryCodeGen.GenerateRecoveryCode()
		if err != nil {
			h.log.Error("failed to generate recovery code", "user_id", userID, "error", err)
			return nil, err
		}
		recoveryCodes = append(recoveryCodes, raw)
		codeID, err := h.idGen.New()
		if err != nil {
			h.log.Error("failed to generate recovery code ID", "user_id", userID, "error", err)
			return nil, err
		}
		records = append(records, domain.RecoveryCode{
			ID:        codeID,
			UserID:    userID,
			CodeHash:  h.tokenHasher.HashToken(raw),
			CreatedAt: now,
		})
	}
	doEnable := func(txCtx context.Context) error {
		if err := h.totp.ReplaceRecoveryCodes(txCtx, userID, records); err != nil {
			return err
		}
		return h.usersWrite.SetMFAEnabledAt(txCtx, userID, &now, now)
	}
	if err := transaction.RunInTx(ctx, h.unitOfWork, doEnable); err != nil {
		h.log.Error("failed to enable MFA", "user_id", userID, "error", err)
		return nil, err
	}
	h.log.Info("MFA enabled", "user_id", userID)
	return &Result{RecoveryCodes: recoveryCodes}, nil
}
