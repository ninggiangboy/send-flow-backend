package forgotpassword

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/usecase"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
)

type Handler struct {
	deps usecase.Deps
	log  *slog.Logger
}

func New(deps usecase.Deps) *Handler {
	return &Handler{deps: deps, log: deps.Logger.With("usecase", "forgot_password")}
}

func (h *Handler) Execute(ctx context.Context, email string, now time.Time) error {
	parsed, err := domain.NewEmailAddress(email)
	if err != nil {
		h.log.Info("password reset requested with invalid email format")
		return nil
	}
	user, err := h.deps.UsersRead.FindByEmail(ctx, parsed.String())
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			h.log.Info("password reset requested for unregistered email")
			return nil
		}
		h.log.Error("failed to lookup user for password reset", "error", err)
		return err
	}
	token, err := usecase.CreatePasswordResetToken(ctx, h.deps, user.ID, now)
	if err != nil || token == "" {
		h.log.Error("failed to create password reset token", "user_id", user.ID, "error", err)
		return err
	}
	if err := usecase.SendPasswordResetEmail(ctx, h.deps, user.Email, usecase.BuildPasswordResetLink(h.deps, token)); err != nil {
		h.log.Error("failed to send password reset email", "user_id", user.ID, "error", err)
		return err
	}
	h.log.Info("password reset email sent", "user_id", user.ID)
	return nil
}