package mfaregenerate

import (
	"context"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/mfatotpenable"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/usecase"
)

type Handler struct {
	inner *mfatotpenable.Handler
	deps  usecase.Deps
	log   *slog.Logger
}

func New(deps usecase.Deps) *Handler {
	return &Handler{inner: mfatotpenable.New(deps), deps: deps, log: deps.Logger.With("usecase", "mfa_regenerate")}
}

func (h *Handler) Execute(ctx context.Context, userID, code string, now time.Time) (*mfatotpenable.Result, error) {
	result, err := h.inner.Execute(ctx, userID, code, now)
	if err != nil {
		return nil, err
	}
	h.log.Info("MFA recovery codes regenerated", "user_id", userID)
	return result, nil
}
