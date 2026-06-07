package resetpassword

import (
	"context"
	"errors"
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
	Token       string
	NewPassword string
	Now         time.Time
}

func New(deps usecase.Deps) *Handler {
	return &Handler{deps: deps, log: deps.Logger.With("usecase", "reset_password")}

}

func (h *Handler) Execute(ctx context.Context, cmd Command) error {
	if err := security.ValidatePasswordPolicy(cmd.NewPassword); err != nil {
		return domain.ErrPasswordPolicy
	}
	record, err := usecase.ConsumeAuthToken(ctx, h.deps, cmd.Token, domain.AuthTokenPurposePasswordReset, cmd.Now)
	if err != nil {
		if errors.Is(err, domain.ErrUnauthorized) || errors.Is(err, domain.ErrNotFound) {
			h.log.Warn("invalid or expired password reset token")
			return domain.ErrResetToken
		}
		h.log.Error("failed to consume password reset token", "error", err)
		return err
	}
	hash, err := h.deps.Hasher.Hash(cmd.NewPassword)
	if err != nil {
		h.log.Error("failed to hash new password", "user_id", record.UserID, "error", err)
		return err
	}
	if err := h.deps.UsersWrite.UpdatePassword(ctx, record.UserID, hash, cmd.Now); err != nil {
		h.log.Error("failed to update password", "user_id", record.UserID, "error", err)
		return err
	}
	if err := h.deps.SessionsWrite.RevokeByUser(ctx, record.UserID, cmd.Now); err != nil {
		h.log.Error("failed to revoke sessions after password reset", "user_id", record.UserID, "error", err)
		return err
	}
	if err := h.deps.AuthTokens.DeleteByUserAndPurpose(ctx, record.UserID, domain.AuthTokenPurposeMFAChallenge); err != nil {
		h.log.Error("failed to clear MFA challenges after password reset", "user_id", record.UserID, "error", err)
		return err
	}
	h.log.Info("password reset completed, all sessions revoked", "user_id", record.UserID)
	return nil
}
