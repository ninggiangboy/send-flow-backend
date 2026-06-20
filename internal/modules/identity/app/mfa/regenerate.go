package mfa

import (
	"context"
	"log/slog"
	"time"
)

type RegenerateHandler struct {
	inner *TOTPEnableHandler
	log   *slog.Logger
}

func NewRegenerateHandler(inner *TOTPEnableHandler, logger *slog.Logger) *RegenerateHandler {
	return &RegenerateHandler{inner: inner, log: logger.With("usecase", "mfa_regenerate")}
}

func (h *RegenerateHandler) Execute(ctx context.Context, userID, code string, now time.Time) (*TOTPEnableResult, error) {
	result, err := h.inner.Execute(ctx, userID, code, now)
	if err != nil {
		return nil, err
	}
	h.log.Info("MFA recovery codes regenerated", "user_id", userID)
	return result, nil
}
