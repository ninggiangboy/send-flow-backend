package mfa

import (
	"context"
	"crypto/subtle"
	"errors"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/shared"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/ports"
)

type MFALoginOptions struct {
	AuthTokens     *shared.AuthTokenService
	UsersWrite     ports.UserWriteRepository
	Totp           ports.TOTPRepository
	TokenHasher    ports.TokenHasher
	TotpVerifier   ports.TOTPCodeVerifier
	SessionFactory *shared.SessionFactory
	Logger         *slog.Logger
}

type MFALoginHandler struct {
	authTokens     *shared.AuthTokenService
	usersWrite     ports.UserWriteRepository
	totp           ports.TOTPRepository
	tokenHasher    ports.TokenHasher
	totpVerifier   ports.TOTPCodeVerifier
	sessionFactory *shared.SessionFactory
	log            *slog.Logger
}

type MFALoginCommand struct {
	ChallengeToken string
	Code           string
	RecoveryCode   string
	IP             string
	UA             string
	Now            time.Time
}

func NewMFALoginHandler(opts MFALoginOptions) *MFALoginHandler {
	return &MFALoginHandler{
		authTokens:     opts.AuthTokens,
		usersWrite:     opts.UsersWrite,
		totp:           opts.Totp,
		tokenHasher:    opts.TokenHasher,
		totpVerifier:   opts.TotpVerifier,
		sessionFactory: opts.SessionFactory,
		log:            opts.Logger.With("usecase", "mfa_login"),
	}
}

func (h *MFALoginHandler) Execute(ctx context.Context, cmd MFALoginCommand) (*shared.SessionContext, error) {
	record, err := h.authTokens.ConsumeToken(ctx, cmd.ChallengeToken, domain.AuthTokenPurposeMFAChallenge, cmd.Now)
	if err != nil {
		if errors.Is(err, domain.ErrUnauthorized) || errors.Is(err, domain.ErrNotFound) {
			h.log.Warn("invalid or expired MFA challenge token")
			return nil, domain.ErrMFAInvalidCode
		}
		h.log.Error("failed to consume MFA challenge token", "error", err)
		return nil, err
	}
	user, err := h.usersWrite.FindByID(ctx, record.UserID)
	if err != nil {
		h.log.Error("failed to find user for MFA login", "user_id", record.UserID, "error", err)
		return nil, err
	}
	if cmd.RecoveryCode != "" {
		codes, err := h.totp.ListRecoveryCodes(ctx, user.ID)
		if err != nil {
			h.log.Error("failed to list recovery codes", "user_id", user.ID, "error", err)
			return nil, err
		}
		rawHash := h.tokenHasher.HashToken(cmd.RecoveryCode)
		for _, code := range codes {
			if code.ConsumedAt == nil && subtle.ConstantTimeCompare([]byte(code.CodeHash), []byte(rawHash)) == 1 {
				if err := h.totp.ConsumeRecoveryCode(ctx, code.ID, cmd.Now); err != nil {
					h.log.Error("failed to consume recovery code", "user_id", user.ID, "error", err)
					return nil, err
				}
				h.log.Info("MFA login completed via recovery code", "user_id", user.ID)
				return h.sessionFactory.NewSession(ctx, shared.NewSessionInput{User: *user, Method: "password_mfa", IP: cmd.IP, UA: cmd.UA, Now: cmd.Now})
			}
		}
		h.log.Warn("invalid MFA recovery code", "user_id", user.ID)
		return nil, domain.ErrMFAInvalidCode
	}
	secret, err := h.totp.FindSecretByUser(ctx, user.ID)
	if err != nil {
		h.log.Warn("failed to find TOTP secret for MFA login", "user_id", user.ID, "error", err)
		return nil, domain.ErrMFAInvalidCode
	}
	if secret == nil {
		h.log.Warn("TOTP secret is nil for MFA login", "user_id", user.ID)
		return nil, domain.ErrMFAInvalidCode
	}
	if !h.totpVerifier.VerifyTOTPCode(secret.Secret, cmd.Code, cmd.Now) {
		h.log.Warn("invalid MFA TOTP code", "user_id", user.ID)
		return nil, domain.ErrMFAInvalidCode
	}
	h.log.Info("MFA login completed via TOTP", "user_id", user.ID)
	return h.sessionFactory.NewSession(ctx, shared.NewSessionInput{User: *user, Method: "password_mfa", IP: cmd.IP, UA: cmd.UA, Now: cmd.Now})
}
