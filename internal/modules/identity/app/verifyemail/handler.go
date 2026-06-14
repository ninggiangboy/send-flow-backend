package verifyemail

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/usecase"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/ports"
)

type Options struct {
	AuthTokens *usecase.AuthTokenService
	UsersWrite ports.UserWriteRepository
	Logger     *slog.Logger
}

type Handler struct {
	authTokens *usecase.AuthTokenService
	usersWrite ports.UserWriteRepository
	log        *slog.Logger
}

func New(opts Options) *Handler {
	return &Handler{
		authTokens: opts.AuthTokens,
		usersWrite: opts.UsersWrite,
		log:        opts.Logger.With("usecase", "verify_email"),
	}
}

func (h *Handler) Execute(ctx context.Context, token string, now time.Time) error {
	record, err := h.authTokens.ConsumeToken(ctx, token, domain.AuthTokenPurposeEmailVerification, now)
	if err != nil {
		if errors.Is(err, domain.ErrUnauthorized) || errors.Is(err, domain.ErrNotFound) {
			h.log.Warn("invalid or expired email verification token")
			return domain.ErrVerificationToken
		}
		h.log.Error("failed to consume email verification token", "error", err)
		return err
	}
	if err := h.usersWrite.MarkEmailVerified(ctx, record.UserID, now); err != nil {
		h.log.Error("failed to mark email as verified", "user_id", record.UserID, "error", err)
		return err
	}
	h.log.Info("email verified", "user_id", record.UserID)
	return nil
}
