package mfatotpdisable

import (
	"context"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/usecase"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/security"
)

type Handler struct {
	deps usecase.Deps
	log  *slog.Logger
}

type Command struct {
	UserID   string
	Password string
	Code     string
	Now      time.Time
}

func New(deps usecase.Deps) *Handler {
	return &Handler{deps: deps, log: deps.Logger.With("usecase", "mfa_totp_disable")}
}

func (h *Handler) Execute(ctx context.Context, cmd Command) error {
	user, err := h.deps.UsersRead.FindByID(ctx, cmd.UserID)
	if err != nil {
		h.log.Error("failed to find user for MFA disable", "user_id", cmd.UserID, "error", err)
		return err
	}
	authorized := false
	if cmd.Password != "" && user.HashedPassword != "" && user.VerifyPassword(cmd.Password, h.deps.Hasher) == nil {
		authorized = true
	}
	if !authorized && cmd.Code != "" {
		secret, err := h.deps.TOTP.FindSecretByUser(ctx, cmd.UserID)
		if err == nil && security.VerifyTOTPCode(secret.Secret, cmd.Code, cmd.Now) {
			authorized = true
		}
	}
	if !authorized {
		h.log.Warn("MFA disable authorization failed", "user_id", cmd.UserID)
		return domain.ErrMFAInvalidCode
	}
	if err := h.deps.UsersWrite.SetMFAEnabledAt(ctx, cmd.UserID, nil, cmd.Now); err != nil {
		h.log.Error("failed to disable MFA", "user_id", cmd.UserID, "error", err)
		return err
	}
	if err := h.deps.TOTP.DeleteSecret(ctx, cmd.UserID); err != nil {
		h.log.Error("failed to delete TOTP secret", "user_id", cmd.UserID, "error", err)
		return err
	}
	if err := h.deps.TOTP.ReplaceRecoveryCodes(ctx, cmd.UserID, nil); err != nil {
		h.log.Error("failed to remove recovery codes", "user_id", cmd.UserID, "error", err)
		return err
	}
	h.log.Info("MFA disabled", "user_id", cmd.UserID)
	return nil
}