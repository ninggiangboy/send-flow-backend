package verifyemail

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
	return &Handler{deps: deps, log: deps.Logger.With("usecase", "verify_email")}
}

func (h *Handler) Execute(ctx context.Context, token string, now time.Time) error {
	record, err := usecase.ConsumeAuthToken(ctx, h.deps, token, domain.AuthTokenPurposeEmailVerification, now)
	if err != nil {
		if errors.Is(err, domain.ErrUnauthorized) || errors.Is(err, domain.ErrNotFound) {
			h.log.Warn("invalid or expired email verification token")
			return domain.ErrVerificationToken
		}
		h.log.Error("failed to consume email verification token", "error", err)
		return err
	}
	if err := h.deps.UsersWrite.MarkEmailVerified(ctx, record.UserID, now); err != nil {
		h.log.Error("failed to mark email as verified", "user_id", record.UserID, "error", err)
		return err
	}
	h.log.Info("email verified", "user_id", record.UserID)
	return nil
}