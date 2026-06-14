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

type userReadStub struct {
	user *domain.User
	err  error
}

func (s *userReadStub) FindByEmail(context.Context, string) (*domain.User, error) {
	return s.user, s.err
}
func (s *userReadStub) FindByID(context.Context, string) (*domain.User, error) { return s.user, s.err }

type hasherStub struct {
	compareErr error
}

func (s *hasherStub) Hash(string) (string, error)  { return "hash", nil }
func (s *hasherStub) Compare(string, string) error { return s.compareErr }

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

func TestExecuteSuccessWithPassword(t *testing.T) {
	now := time.Now()
	h := New(Options{
		UsersRead: &userReadStub{
			user: &domain.User{ID: "u1", HashedPassword: "hash"},
		},
		Hasher:       &hasherStub{compareErr: nil},
		Totp:         &totpStub{},
		TotpVerifier: &totpVerifierStub{},
		UsersWrite:   &userWriteStub{},
		UnitOfWork:   noopTx{},
		Logger:       testLogger,
	})
	err := h.Execute(context.Background(), Command{UserID: "u1", Password: "password", Now: now})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExecuteSuccessWithCode(t *testing.T) {
	now := time.Now()
	h := New(Options{
		UsersRead: &userReadStub{
			user: &domain.User{ID: "u1", HashedPassword: ""},
		},
		Hasher:       &hasherStub{compareErr: errors.New("no match")},
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

func TestExecuteUserNotFound(t *testing.T) {
	now := time.Now()
	h := New(Options{
		UsersRead:    &userReadStub{err: domain.ErrNotFound},
		Hasher:       &hasherStub{},
		Totp:         &totpStub{},
		TotpVerifier: &totpVerifierStub{},
		UsersWrite:   &userWriteStub{},
		UnitOfWork:   noopTx{},
		Logger:       testLogger,
	})
	err := h.Execute(context.Background(), Command{UserID: "u1", Now: now})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestExecuteAuthorizationFails(t *testing.T) {
	now := time.Now()
	h := New(Options{
		UsersRead: &userReadStub{
			user: &domain.User{ID: "u1", HashedPassword: "hash"},
		},
		Hasher:       &hasherStub{compareErr: errors.New("wrong password")},
		Totp:         &totpStub{findErr: errors.New("not found")},
		TotpVerifier: &totpVerifierStub{valid: false},
		UsersWrite:   &userWriteStub{},
		UnitOfWork:   noopTx{},
		Logger:       testLogger,
	})
	err := h.Execute(context.Background(), Command{UserID: "u1", Password: "wrong", Code: "wrong", Now: now})
	if !errors.Is(err, domain.ErrMFAInvalidCode) {
		t.Fatalf("expected ErrMFAInvalidCode, got %v", err)
	}
}

func TestExecuteTxFails(t *testing.T) {
	upstreamErr := errors.New("db error")
	now := time.Now()
	h := New(Options{
		UsersRead: &userReadStub{
			user: &domain.User{ID: "u1", HashedPassword: "hash"},
		},
		Hasher:       &hasherStub{compareErr: nil},
		Totp:         &totpStub{},
		TotpVerifier: &totpVerifierStub{},
		UsersWrite:   &userWriteStub{err: upstreamErr},
		UnitOfWork:   noopTx{},
		Logger:       testLogger,
	})
	err := h.Execute(context.Background(), Command{UserID: "u1", Password: "password", Now: now})
	if !errors.Is(err, upstreamErr) {
		t.Fatalf("expected upstream error, got %v", err)
	}
}
