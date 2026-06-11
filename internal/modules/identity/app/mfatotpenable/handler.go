package mfatotpenable

import (
	"context"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/usecase"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/transaction"
)

type Handler struct {
	deps usecase.Deps
	log  *slog.Logger
}

type Result struct {
	RecoveryCodes []string `json:"recovery_codes"`
}

func New(deps usecase.Deps) *Handler {
	return &Handler{deps: deps, log: deps.Logger.With("usecase", "mfa_totp_enable")}
}

func (h *Handler) Execute(ctx context.Context, userID, code string, now time.Time) (*Result, error) {
	secret, err := h.deps.TOTP.FindSecretByUser(ctx, userID)
	if err != nil || !h.deps.TOTPVerifier.VerifyTOTPCode(secret.Secret, code, now) {
		h.log.Warn("invalid TOTP code during MFA enable", "user_id", userID)
		return nil, domain.ErrMFAInvalidCode
	}
	recoveryCodes := make([]string, 0, 8)
	records := make([]domain.RecoveryCode, 0, 8)
	for range 8 {
		raw, err := h.deps.RecoveryCodeGen.GenerateRecoveryCode()
		if err != nil {
			h.log.Error("failed to generate recovery code", "user_id", userID, "error", err)
			return nil, err
		}
		recoveryCodes = append(recoveryCodes, raw)
		codeID, err := h.deps.IDGen.New()
		if err != nil {
			h.log.Error("failed to generate recovery code ID", "user_id", userID, "error", err)
			return nil, err
		}
		records = append(records, domain.RecoveryCode{
			ID:        codeID,
			UserID:    userID,
			CodeHash:  h.deps.TokenHasher.HashToken(raw),
			CreatedAt: now,
		})
	}
	doEnable := func(txCtx context.Context) error {
		if err := h.deps.TOTP.ReplaceRecoveryCodes(txCtx, userID, records); err != nil {
			return err
		}
		return h.deps.UsersWrite.SetMFAEnabledAt(txCtx, userID, &now, now)
	}
	if err := transaction.RunInTx(ctx, h.deps.UnitOfWork, doEnable); err != nil {
		h.log.Error("failed to enable MFA", "user_id", userID, "error", err)
		return nil, err
	}
	h.log.Info("MFA enabled", "user_id", userID)
	return &Result{RecoveryCodes: recoveryCodes}, nil
}
