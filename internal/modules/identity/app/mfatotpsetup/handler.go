package mfatotpsetup

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/ports"
)

type Options struct {
	UsersRead     ports.UserReadRepository
	TotpSecretGen ports.TOTPSecretGenerator
	Totp          ports.TOTPRepository
	Logger        *slog.Logger
}

type Handler struct {
	usersRead     ports.UserReadRepository
	totpSecretGen ports.TOTPSecretGenerator
	totp          ports.TOTPRepository
	log           *slog.Logger
}

type Result struct {
	Secret     string `json:"secret"`
	OTPAuthURL string `json:"otpauth_url"`
}

func New(opts Options) *Handler {
	return &Handler{
		usersRead:     opts.UsersRead,
		totpSecretGen: opts.TotpSecretGen,
		totp:          opts.Totp,
		log:           opts.Logger.With("usecase", "mfa_totp_setup"),
	}
}

func (h *Handler) Execute(ctx context.Context, userID string, now time.Time) (*Result, error) {
	user, err := h.usersRead.FindByID(ctx, userID)
	if err != nil {
		h.log.Error("failed to find user for MFA setup", "user_id", userID, "error", err)
		return nil, err
	}
	secret, err := h.totpSecretGen.GenerateTOTPSecret()
	if err != nil {
		h.log.Error("failed to generate TOTP secret", "user_id", userID, "error", err)
		return nil, err
	}
	if err := h.totp.UpsertSecret(ctx, domain.TOTPSecret{UserID: userID, Secret: secret, CreatedAt: now, UpdatedAt: now}); err != nil {
		h.log.Error("failed to save TOTP secret", "user_id", userID, "error", err)
		return nil, err
	}
	h.log.Info("MFA TOTP setup initiated", "user_id", userID)
	label := url.QueryEscape(strings.TrimSpace(user.Email))
	return &Result{
		Secret:     secret,
		OTPAuthURL: fmt.Sprintf("otpauth://totp/SendFlow:%s?secret=%s&issuer=SendFlow", label, secret),
	}, nil
}
