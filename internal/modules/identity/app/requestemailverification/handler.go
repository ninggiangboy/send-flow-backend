package requestemailverification

import (
	"context"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/usecase"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/ports"
)

type Options struct {
	UsersRead       ports.UserReadRepository
	AuthTokens      *usecase.AuthTokenService
	Notifications   *usecase.NotificationService
	VerificationTTL time.Duration
	Logger          *slog.Logger
}

type Handler struct {
	usersRead       ports.UserReadRepository
	authTokens      *usecase.AuthTokenService
	notifications   *usecase.NotificationService
	verificationTTL time.Duration
	log             *slog.Logger
}

func New(opts Options) *Handler {
	return &Handler{
		usersRead:       opts.UsersRead,
		authTokens:      opts.AuthTokens,
		notifications:   opts.Notifications,
		verificationTTL: opts.VerificationTTL,
		log:             opts.Logger.With("usecase", "request_email_verification"),
	}
}

func (h *Handler) Execute(ctx context.Context, userID string, now time.Time) error {
	user, err := h.usersRead.FindByID(ctx, userID)
	if err != nil || user.EmailVerified() {
		return err
	}
	token, err := h.authTokens.CreateToken(ctx, user.ID, domain.AuthTokenPurposeEmailVerification, h.verificationTTL, now)
	if err != nil || token == "" {
		h.log.Error("failed to create email verification token", "user_id", userID, "error", err)
		return err
	}
	if err := h.notifications.SendVerificationEmail(ctx, user.Email, h.notifications.BuildEmailVerificationLink(token)); err != nil {
		h.log.Error("failed to send verification email", "user_id", userID, "error", err)
		return err
	}
	h.log.Info("verification email sent", "user_id", userID)
	return nil
}
