package mfatotpenable

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/usecase"
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
	h := New(usecase.Deps{
		TOTP:            &totpStub{secret: &domain.TOTPSecret{Secret: "JBSWY3DPEHPK3PXP"}},
		TOTPVerifier:    &totpVerifierStub{valid: true},
		RecoveryCodeGen: &recoveryCodeGenStub{code: "RC-123456"},
		IDGen:           &idGenStub{id: "rc-1"},
		TokenHasher:     tokenHasherStub{},
		UsersWrite:      &userWriteStub{},
		UnitOfWork:      noopTx{},
		Logger:          testLogger,
	})
	res, err := h.Execute(context.Background(), "u1", "123456", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.RecoveryCodes) != 8 {
		t.Fatalf("expected 8 recovery codes, got %d", len(res.RecoveryCodes))
	}
}

func TestExecuteFindSecretFails(t *testing.T) {
	now := time.Now()
	h := New(usecase.Deps{
		TOTP:            &totpStub{findErr: errors.New("not found")},
		TOTPVerifier:    &totpVerifierStub{valid: true},
		RecoveryCodeGen: &recoveryCodeGenStub{code: "RC-123456"},
		IDGen:           &idGenStub{id: "rc-1"},
		TokenHasher:     tokenHasherStub{},
		UsersWrite:      &userWriteStub{},
		UnitOfWork:      noopTx{},
		Logger:          testLogger,
	})
	_, err := h.Execute(context.Background(), "u1", "123456", now)
	if !errors.Is(err, domain.ErrMFAInvalidCode) {
		t.Fatalf("expected ErrMFAInvalidCode, got %v", err)
	}
}

func TestExecuteVerifyCodeFails(t *testing.T) {
	now := time.Now()
	h := New(usecase.Deps{
		TOTP:            &totpStub{secret: &domain.TOTPSecret{Secret: "JBSWY3DPEHPK3PXP"}},
		TOTPVerifier:    &totpVerifierStub{valid: false},
		RecoveryCodeGen: &recoveryCodeGenStub{code: "RC-123456"},
		IDGen:           &idGenStub{id: "rc-1"},
		TokenHasher:     tokenHasherStub{},
		UsersWrite:      &userWriteStub{},
		UnitOfWork:      noopTx{},
		Logger:          testLogger,
	})
	_, err := h.Execute(context.Background(), "u1", "wrong", now)
	if !errors.Is(err, domain.ErrMFAInvalidCode) {
		t.Fatalf("expected ErrMFAInvalidCode, got %v", err)
	}
}

func TestExecuteRecoveryCodeGenFails(t *testing.T) {
	upstreamErr := errors.New("rng failure")
	now := time.Now()
	h := New(usecase.Deps{
		TOTP:            &totpStub{secret: &domain.TOTPSecret{Secret: "JBSWY3DPEHPK3PXP"}},
		TOTPVerifier:    &totpVerifierStub{valid: true},
		RecoveryCodeGen: &recoveryCodeGenStub{err: upstreamErr},
		IDGen:           &idGenStub{id: "rc-1"},
		TokenHasher:     tokenHasherStub{},
		UsersWrite:      &userWriteStub{},
		UnitOfWork:      noopTx{},
		Logger:          testLogger,
	})
	_, err := h.Execute(context.Background(), "u1", "123456", now)
	if !errors.Is(err, upstreamErr) {
		t.Fatalf("expected upstream error, got %v", err)
	}
}

func TestExecuteIDGenFails(t *testing.T) {
	upstreamErr := errors.New("id gen failure")
	now := time.Now()
	h := New(usecase.Deps{
		TOTP:            &totpStub{secret: &domain.TOTPSecret{Secret: "JBSWY3DPEHPK3PXP"}},
		TOTPVerifier:    &totpVerifierStub{valid: true},
		RecoveryCodeGen: &recoveryCodeGenStub{code: "RC-123456"},
		IDGen:           &idGenStub{err: upstreamErr},
		TokenHasher:     tokenHasherStub{},
		UsersWrite:      &userWriteStub{},
		UnitOfWork:      noopTx{},
		Logger:          testLogger,
	})
	_, err := h.Execute(context.Background(), "u1", "123456", now)
	if !errors.Is(err, upstreamErr) {
		t.Fatalf("expected upstream error, got %v", err)
	}
}

func TestExecuteReplaceRecoveryCodesFails(t *testing.T) {
	upstreamErr := errors.New("db error")
	now := time.Now()
	h := New(usecase.Deps{
		TOTP:            &totpStub{secret: &domain.TOTPSecret{Secret: "JBSWY3DPEHPK3PXP"}, replaceErr: upstreamErr},
		TOTPVerifier:    &totpVerifierStub{valid: true},
		RecoveryCodeGen: &recoveryCodeGenStub{code: "RC-123456"},
		IDGen:           &idGenStub{id: "rc-1"},
		TokenHasher:     tokenHasherStub{},
		UsersWrite:      &userWriteStub{},
		UnitOfWork:      noopTx{},
		Logger:          testLogger,
	})
	_, err := h.Execute(context.Background(), "u1", "123456", now)
	if !errors.Is(err, upstreamErr) {
		t.Fatalf("expected upstream error, got %v", err)
	}
}

func TestExecuteSetMFAEnabledAtFails(t *testing.T) {
	upstreamErr := errors.New("db error")
	now := time.Now()
	h := New(usecase.Deps{
		TOTP:            &totpStub{secret: &domain.TOTPSecret{Secret: "JBSWY3DPEHPK3PXP"}},
		TOTPVerifier:    &totpVerifierStub{valid: true},
		RecoveryCodeGen: &recoveryCodeGenStub{code: "RC-123456"},
		IDGen:           &idGenStub{id: "rc-1"},
		TokenHasher:     tokenHasherStub{},
		UsersWrite:      &userWriteStub{err: upstreamErr},
		UnitOfWork:      noopTx{},
		Logger:          testLogger,
	})
	_, err := h.Execute(context.Background(), "u1", "123456", now)
	if !errors.Is(err, upstreamErr) {
		t.Fatalf("expected upstream error, got %v", err)
	}
}
