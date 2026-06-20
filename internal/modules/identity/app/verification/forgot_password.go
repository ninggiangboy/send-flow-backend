package verification

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/shared"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/ports"
)

type ForgotPasswordOptions struct {
	UsersRead        ports.UserReadRepository
	AuthTokens       *shared.AuthTokenService
	Notifications    *shared.NotificationService
	PasswordResetTTL time.Duration
	Logger           *slog.Logger
}

type ForgotPasswordHandler struct {
	usersRead        ports.UserReadRepository
	authTokens       *shared.AuthTokenService
	notifications    *shared.NotificationService
	passwordResetTTL time.Duration
	log              *slog.Logger
}

func NewForgotPasswordHandler(opts ForgotPasswordOptions) *ForgotPasswordHandler {
	return &ForgotPasswordHandler{
		usersRead:        opts.UsersRead,
		authTokens:       opts.AuthTokens,
		notifications:    opts.Notifications,
		passwordResetTTL: opts.PasswordResetTTL,
		log:              opts.Logger.With("usecase", "forgot_password"),
	}
}

func (h *ForgotPasswordHandler) Execute(ctx context.Context, email string, now time.Time) error {
	parsed, err := domain.NewEmailAddress(email)
	if err != nil {
		h.log.Info("password reset requested with invalid email format")
		return nil
	}
	user, err := h.usersRead.FindByEmail(ctx, parsed.String())
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			h.log.Debug("password reset requested")
			return nil
		}
		h.log.Error("failed to lookup user for password reset", "error", err)
		return err
	}
	token, err := h.authTokens.CreateToken(ctx, user.ID, domain.AuthTokenPurposePasswordReset, h.passwordResetTTL, now)
	if err != nil || token == "" {
		h.log.Error("failed to create password reset token", "user_id", user.ID, "error", err)
		return err
	}
	if err := h.notifications.SendPasswordResetEmail(ctx, user.Email, h.notifications.BuildPasswordResetLink(token)); err != nil {
		h.log.Error("failed to send password reset email", "user_id", user.ID, "error", err)
		return err
	}
	h.log.Info("password reset email sent", "user_id", user.ID)
	return nil
}
