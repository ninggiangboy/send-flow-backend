package mfatotpsetup

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
)

var testLogger = slog.Default()

type userReadStub struct {
	user *domain.User
	err  error
}

func (s *userReadStub) FindByEmail(context.Context, string) (*domain.User, error) {
	return s.user, s.err
}
func (s *userReadStub) FindByID(context.Context, string) (*domain.User, error) { return s.user, s.err }

type totpStub struct {
	upsertErr error
}

func (s *totpStub) UpsertSecret(_ context.Context, sec domain.TOTPSecret) error { return s.upsertErr }
func (s *totpStub) FindSecretByUser(_ context.Context, userID string) (*domain.TOTPSecret, error) {
	return nil, nil
}
func (s *totpStub) DeleteSecret(_ context.Context, userID string) error { return nil }
func (s *totpStub) ReplaceRecoveryCodes(_ context.Context, userID string, codes []domain.RecoveryCode) error {
	return nil
}
func (s *totpStub) ListRecoveryCodes(_ context.Context, userID string) ([]domain.RecoveryCode, error) {
	return nil, nil
}
func (s *totpStub) ConsumeRecoveryCode(_ context.Context, codeID string, at time.Time) error {
	return nil
}

type totpSecretGenStub struct {
	secret string
	err    error
}

func (s *totpSecretGenStub) GenerateTOTPSecret() (string, error) { return s.secret, s.err }

func TestExecuteSuccess(t *testing.T) {
	h := New(Options{
		UsersRead:     &userReadStub{user: &domain.User{ID: "u1", Email: "test@example.com"}},
		Totp:          &totpStub{},
		TotpSecretGen: &totpSecretGenStub{secret: "JBSWY3DPEHPK3PXP"},
		Logger:        testLogger,
	})
	res, err := h.Execute(context.Background(), "u1", time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Secret != "JBSWY3DPEHPK3PXP" {
		t.Fatalf("expected secret JBSWY3DPEHPK3PXP, got %s", res.Secret)
	}
	if res.OTPAuthURL == "" {
		t.Fatal("expected non-empty OTPAuthURL")
	}
}

func TestExecuteUserNotFound(t *testing.T) {
	h := New(Options{
		UsersRead:     &userReadStub{err: domain.ErrNotFound},
		Totp:          &totpStub{},
		TotpSecretGen: &totpSecretGenStub{secret: "JBSWY3DPEHPK3PXP"},
		Logger:        testLogger,
	})
	_, err := h.Execute(context.Background(), "u1", time.Now())
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestExecuteGenerateSecretFails(t *testing.T) {
	upstreamErr := errors.New("rng failure")
	h := New(Options{
		UsersRead:     &userReadStub{user: &domain.User{ID: "u1"}},
		Totp:          &totpStub{},
		TotpSecretGen: &totpSecretGenStub{err: upstreamErr},
		Logger:        testLogger,
	})
	_, err := h.Execute(context.Background(), "u1", time.Now())
	if !errors.Is(err, upstreamErr) {
		t.Fatalf("expected upstream error, got %v", err)
	}
}

func TestExecuteUpsertSecretFails(t *testing.T) {
	upstreamErr := errors.New("db error")
	h := New(Options{
		UsersRead:     &userReadStub{user: &domain.User{ID: "u1"}},
		Totp:          &totpStub{upsertErr: upstreamErr},
		TotpSecretGen: &totpSecretGenStub{secret: "JBSWY3DPEHPK3PXP"},
		Logger:        testLogger,
	})
	_, err := h.Execute(context.Background(), "u1", time.Now())
	if !errors.Is(err, upstreamErr) {
		t.Fatalf("expected upstream error, got %v", err)
	}
}
