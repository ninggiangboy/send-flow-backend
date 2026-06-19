package mfatotpdisable

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
)

var testLogger = slog.Default()

type noopTx struct{}

func (noopTx) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error { return fn(ctx) }

type totpStub struct {
	secret     *domain.TOTPSecret
	findErr    error
	deleteErr  error
	replaceErr error
}

func (s *totpStub) UpsertSecret(_ context.Context, sec domain.TOTPSecret) error { return nil }
func (s *totpStub) FindSecretByUser(_ context.Context, userID string) (*domain.TOTPSecret, error) {
	return s.secret, s.findErr
}
func (s *totpStub) DeleteSecret(_ context.Context, userID string) error { return s.deleteErr }
func (s *totpStub) ReplaceRecoveryCodes(_ context.Context, userID string, codes []domain.RecoveryCode) error {
	return s.replaceErr
}
func (s *totpStub) ListRecoveryCodes(_ context.Context, userID string) ([]domain.RecoveryCode, error) {
	return nil, nil
}
func (s *totpStub) ConsumeRecoveryCode(_ context.Context, codeID string, at time.Time) error {
	return nil
}

type totpVerifierStub struct {
	valid bool
}

func (s *totpVerifierStub) VerifyTOTPCode(secret, code string, now time.Time) bool { return s.valid }

type userWriteStub struct {
	err error
}

func (s *userWriteStub) Create(_ context.Context, u domain.User) error { return s.err }
func (s *userWriteStub) UpdatePassword(context.Context, string, string, time.Time) error {
	return nil
}
func (s *userWriteStub) MarkEmailVerified(context.Context, string, time.Time) error { return nil }
func (s *userWriteStub) SetMFAEnabledAt(context.Context, string, *time.Time, time.Time) error {
	return s.err
}

func TestExecuteSuccessWithCode(t *testing.T) {
	now := time.Now()
	h := New(Options{
		Totp:         &totpStub{secret: &domain.TOTPSecret{Secret: "JBSWY3DPEHPK3PXP"}},
		TotpVerifier: &totpVerifierStub{valid: true},
		UsersWrite:   &userWriteStub{},
		UnitOfWork:   noopTx{},
		Logger:       testLogger,
	})
	err := h.Execute(context.Background(), Command{UserID: "u1", Code: "123456", Now: now})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExecuteFailsWhenCodeEmpty(t *testing.T) {
	h := New(Options{
		Totp:         &totpStub{},
		TotpVerifier: &totpVerifierStub{},
		UsersWrite:   &userWriteStub{},
		UnitOfWork:   noopTx{},
		Logger:       testLogger,
	})
	err := h.Execute(context.Background(), Command{UserID: "u1", Now: time.Now()})
	if !errors.Is(err, domain.ErrMFAInvalidCode) {
		t.Fatalf("expected ErrMFAInvalidCode for empty code, got %v", err)
	}
}

func TestExecuteFailsWhenSecretNotFound(t *testing.T) {
	now := time.Now()
	h := New(Options{
		Totp:         &totpStub{findErr: errors.New("not found")},
		TotpVerifier: &totpVerifierStub{},
		UsersWrite:   &userWriteStub{},
		UnitOfWork:   noopTx{},
		Logger:       testLogger,
	})
	err := h.Execute(context.Background(), Command{UserID: "u1", Code: "123456", Now: now})
	if !errors.Is(err, domain.ErrMFAInvalidCode) {
		t.Fatalf("expected ErrMFAInvalidCode, got %v", err)
	}
}

func TestExecuteAuthorizationFails(t *testing.T) {
	now := time.Now()
	h := New(Options{
		Totp:         &totpStub{secret: &domain.TOTPSecret{Secret: "JBSWY3DPEHPK3PXP"}},
		TotpVerifier: &totpVerifierStub{valid: false},
		UsersWrite:   &userWriteStub{},
		UnitOfWork:   noopTx{},
		Logger:       testLogger,
	})
	err := h.Execute(context.Background(), Command{UserID: "u1", Code: "wrong", Now: now})
	if !errors.Is(err, domain.ErrMFAInvalidCode) {
		t.Fatalf("expected ErrMFAInvalidCode, got %v", err)
	}
}

func TestExecuteTxFails(t *testing.T) {
	upstreamErr := errors.New("db error")
	now := time.Now()
	h := New(Options{
		Totp:         &totpStub{secret: &domain.TOTPSecret{Secret: "JBSWY3DPEHPK3PXP"}},
		TotpVerifier: &totpVerifierStub{valid: true},
		UsersWrite:   &userWriteStub{err: upstreamErr},
		UnitOfWork:   noopTx{},
		Logger:       testLogger,
	})
	err := h.Execute(context.Background(), Command{UserID: "u1", Code: "123456", Now: now})
	if !errors.Is(err, upstreamErr) {
		t.Fatalf("expected upstream error, got %v", err)
	}
}
