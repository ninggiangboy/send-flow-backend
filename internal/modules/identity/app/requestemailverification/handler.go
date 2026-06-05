package requestemailverification

import (
	"context"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/usecase"
)

type Handler struct {
	deps usecase.Deps
	log  *slog.Logger
}

func New(deps usecase.Deps) *Handler {
	return &Handler{deps: deps, log: deps.Logger.With("usecase", "request_email_verification")}
}

func (h *Handler) Execute(ctx context.Context, userID string, now time.Time) error {
	user, err := h.deps.UsersRead.FindByID(ctx, userID)
	if err != nil || user.EmailVerified() {
		return err
	}
	token, err := usecase.CreateEmailVerificationToken(ctx, h.deps, *user, now)
	if err != nil || token == "" {
		h.log.Error("failed to create email verification token", "user_id", userID, "error", err)
		return err
	}
	if err := usecase.SendVerificationEmail(ctx, h.deps, user.Email, usecase.BuildEmailVerificationLink(h.deps, token)); err != nil {
		h.log.Error("failed to send verification email", "user_id", userID, "error", err)
		return err
	}
	h.log.Info("verification email sent", "user_id", userID)
	return nil
}