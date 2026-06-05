package mfatotpenable

import (
	"context"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/usecase"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/id"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/security"
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
	if err != nil || !security.VerifyTOTPCode(secret.Secret, code, now) {
		h.log.Warn("invalid TOTP code during MFA enable", "user_id", userID)
		return nil, domain.ErrMFAInvalidCode
	}
	recoveryCodes := make([]string, 0, 8)
	records := make([]domain.RecoveryCode, 0, 8)
	for range 8 {
		raw, err := security.GenerateRecoveryCode()
		if err != nil {
			h.log.Error("failed to generate recovery code", "user_id", userID, "error", err)
			return nil, err
		}
		recoveryCodes = append(recoveryCodes, raw)
		records = append(records, domain.RecoveryCode{
			ID:        id.Must(id.NewUUIDGenerator()),
			UserID:    userID,
			CodeHash:  security.HashToken(raw),
			CreatedAt: now,
		})
	}
	if err := h.deps.TOTP.ReplaceRecoveryCodes(ctx, userID, records); err != nil {
		h.log.Error("failed to save recovery codes", "user_id", userID, "error", err)
		return nil, err
	}
	if err := h.deps.UsersWrite.SetMFAEnabledAt(ctx, userID, &now, now); err != nil {
		h.log.Error("failed to enable MFA", "user_id", userID, "error", err)
		return nil, err
	}
	h.log.Info("MFA enabled", "user_id", userID)
	return &Result{RecoveryCodes: recoveryCodes}, nil
}