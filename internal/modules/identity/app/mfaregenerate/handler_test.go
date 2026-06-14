package mfaregenerate

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/mfatotpenable"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
)

var testLogger = slog.Default()

type noopTx struct{}

func (noopTx) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error { return fn(ctx) }

type totpStub struct {
	secret     *domain.TOTPSecret
	findErr    error
	replaceErr error
}

func (s *totpStub) UpsertSecret(_ context.Context, sec domain.TOTPSecret) error { return nil }
func (s *totpStub) FindSecretByUser(_ context.Context, userID string) (*domain.TOTPSecret, error) {
	return s.secret, s.findErr
}
func (s *totpStub) DeleteSecret(_ context.Context, userID string) error { return nil }
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

type recoveryCodeGenStub struct {
	code string
	err  error
}

func (s *recoveryCodeGenStub) GenerateRecoveryCode() (string, error) { return s.code, s.err }

type idGenStub struct {
	id  string
	err error
}

func (s *idGenStub) New() (string, error) { return s.id, s.err }

type tokenHasherStub struct{}

func (tokenHasherStub) HashToken(token string) string { return "hashed:" + token }

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

func TestExecuteSuccess(t *testing.T) {
	now := time.Now()
	enableHandler := mfatotpenable.New(mfatotpenable.Options{
		Totp:            &totpStub{secret: &domain.TOTPSecret{Secret: "JBSWY3DPEHPK3PXP"}},
		TotpVerifier:    &totpVerifierStub{valid: true},
		RecoveryCodeGen: &recoveryCodeGenStub{code: "RC-123456"},
		IdGen:           &idGenStub{id: "rc-1"},
		TokenHasher:     tokenHasherStub{},
		UsersWrite:      &userWriteStub{},
		UnitOfWork:      noopTx{},
		Logger:          testLogger,
	})
	h := New(enableHandler, testLogger)
	res, err := h.Execute(context.Background(), "u1", "123456", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.RecoveryCodes) != 8 {
		t.Fatalf("expected 8 recovery codes, got %d", len(res.RecoveryCodes))
	}
}

func TestExecuteInvalidCode(t *testing.T) {
	now := time.Now()
	enableHandler := mfatotpenable.New(mfatotpenable.Options{
		Totp:            &totpStub{secret: &domain.TOTPSecret{Secret: "JBSWY3DPEHPK3PXP"}},
		TotpVerifier:    &totpVerifierStub{valid: false},
		RecoveryCodeGen: &recoveryCodeGenStub{code: "RC-123456"},
		IdGen:           &idGenStub{id: "rc-1"},
		TokenHasher:     tokenHasherStub{},
		UsersWrite:      &userWriteStub{},
		UnitOfWork:      noopTx{},
		Logger:          testLogger,
	})
	h := New(enableHandler, testLogger)
	_, err := h.Execute(context.Background(), "u1", "wrong", now)
	if !errors.Is(err, domain.ErrMFAInvalidCode) {
		t.Fatalf("expected ErrMFAInvalidCode, got %v", err)
	}
}
