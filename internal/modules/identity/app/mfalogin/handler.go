package mfalogin

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
	deps       usecase.Deps
	newSession usecase.NewSession
	log        *slog.Logger
}

type Command struct {
	ChallengeToken string
	Code           string
	RecoveryCode   string
	IP             string
	UA             string
	Now            time.Time
}

func New(deps usecase.Deps, newSession usecase.NewSession) *Handler {
	return &Handler{deps: deps, newSession: newSession, log: deps.Logger.With("usecase", "mfa_login")}
}

func (h *Handler) Execute(ctx context.Context, cmd Command) (*usecase.SessionContext, error) {
	record, err := usecase.ConsumeAuthToken(ctx, h.deps, cmd.ChallengeToken, domain.AuthTokenPurposeMFAChallenge, cmd.Now)
	if err != nil {
		if errors.Is(err, domain.ErrUnauthorized) || errors.Is(err, domain.ErrNotFound) {
			h.log.Warn("invalid or expired MFA challenge token")
			return nil, domain.ErrMFAInvalidCode
		}
		h.log.Error("failed to consume MFA challenge token", "error", err)
		return nil, err
	}
	user, err := h.deps.UsersRead.FindByID(ctx, record.UserID)
	if err != nil {
		h.log.Error("failed to find user for MFA login", "user_id", record.UserID, "error", err)
		return nil, err
	}
	if cmd.RecoveryCode != "" {
		codes, err := h.deps.TOTP.ListRecoveryCodes(ctx, user.ID)
		if err != nil {
			h.log.Error("failed to list recovery codes", "user_id", user.ID, "error", err)
			return nil, err
		}
		rawHash := security.HashToken(cmd.RecoveryCode)
		for _, code := range codes {
			if code.ConsumedAt == nil && code.CodeHash == rawHash {
				if err := h.deps.TOTP.ConsumeRecoveryCode(ctx, code.ID, cmd.Now); err != nil {
					h.log.Error("failed to consume recovery code", "user_id", user.ID, "error", err)
					return nil, err
				}
				h.log.Info("MFA login completed via recovery code", "user_id", user.ID)
				return h.newSession(ctx, usecase.NewSessionInput{User: *user, Method: "password_mfa", IP: cmd.IP, UA: cmd.UA, Now: cmd.Now})
			}
		}
		h.log.Warn("invalid MFA recovery code", "user_id", user.ID)
		return nil, domain.ErrMFAInvalidCode
	}
	secret, err := h.deps.TOTP.FindSecretByUser(ctx, user.ID)
	if err != nil || !security.VerifyTOTPCode(secret.Secret, cmd.Code, cmd.Now) {
		h.log.Warn("invalid MFA TOTP code", "user_id", user.ID)
		return nil, domain.ErrMFAInvalidCode
	}
	h.log.Info("MFA login completed via TOTP", "user_id", user.ID)
	return h.newSession(ctx, usecase.NewSessionInput{User: *user, Method: "password_mfa", IP: cmd.IP, UA: cmd.UA, Now: cmd.Now})
}