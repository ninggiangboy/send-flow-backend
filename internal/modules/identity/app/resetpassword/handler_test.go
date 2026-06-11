package resetpassword

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

type passwordValidatorStub struct {
	err error
}

func (s *passwordValidatorStub) Validate(password string) error { return s.err }

type authTokenRepoStub struct {
	token *domain.AuthToken
	err   error
}

func (s *authTokenRepoStub) FindByHash(ctx context.Context, purpose, tokenHash string) (*domain.AuthToken, error) {
	return s.token, s.err
}
func (s *authTokenRepoStub) Consume(ctx context.Context, tokenID string, at time.Time) error {
	return nil
}
func (s *authTokenRepoStub) Create(ctx context.Context, token domain.AuthToken) error { return nil }
func (s *authTokenRepoStub) DeleteByUserAndPurpose(ctx context.Context, userID, purpose string) error {
	return nil
}

type tokenHasherStub struct{}

func (tokenHasherStub) HashToken(token string) string { return "hashed-" + token }

type hasherStub struct {
	hash string
	err  error
}

func (s *hasherStub) Hash(password string) (string, error) { return s.hash, s.err }
func (s *hasherStub) Compare(hash, password string) error  { return nil }

type userWriteStub struct{}

func (s *userWriteStub) UpdatePassword(ctx context.Context, userID, hashedPassword string, at time.Time) error {
	return nil
}
func (s *userWriteStub) MarkEmailVerified(ctx context.Context, userID string, at time.Time) error {
	return nil
}
func (s *userWriteStub) SetMFAEnabledAt(ctx context.Context, userID string, enabledAt *time.Time, at time.Time) error {
	return nil
}
func (s *userWriteStub) Create(ctx context.Context, user domain.User) error { return nil }

type sessionsWriteStub struct{}

func (s *sessionsWriteStub) RevokeByUser(ctx context.Context, userID string, now time.Time) error {
	return nil
}
func (s *sessionsWriteStub) Create(ctx context.Context, session domain.Session) error { return nil }
func (s *sessionsWriteStub) RevokeByID(ctx context.Context, sessionID string, now time.Time) error {
	return nil
}
func (s *sessionsWriteStub) RotateTokens(ctx context.Context, sessionID, accessJTI, refreshJTI string, expiresAt, now time.Time) error {
	return nil
}

func TestExecute_PasswordPolicyViolation(t *testing.T) {
	h := New(usecase.Deps{
		PasswordValidator: &passwordValidatorStub{err: errors.New("weak")},
		Logger:            testLogger,
	})
	err := h.Execute(context.Background(), Command{Token: "t", NewPassword: "weak", Now: time.Now()})
	if !errors.Is(err, domain.ErrPasswordPolicy) {
		t.Fatalf("expected ErrPasswordPolicy, got %v", err)
	}
}

func TestExecute_InvalidResetToken(t *testing.T) {
	h := New(usecase.Deps{
		PasswordValidator: &passwordValidatorStub{},
		AuthTokens:        &authTokenRepoStub{err: domain.ErrNotFound},
		TokenHasher:       tokenHasherStub{},
		Logger:            testLogger,
	})
	err := h.Execute(context.Background(), Command{Token: "t", NewPassword: "StrongPassword123!", Now: time.Now()})
	if !errors.Is(err, domain.ErrResetToken) {
		t.Fatalf("expected ErrResetToken, got %v", err)
	}
}

func TestExecute_ConsumeTokenFails(t *testing.T) {
	h := New(usecase.Deps{
		PasswordValidator: &passwordValidatorStub{},
		AuthTokens:        &authTokenRepoStub{err: errors.New("unexpected")},
		TokenHasher:       tokenHasherStub{},
		Logger:            testLogger,
	})
	err := h.Execute(context.Background(), Command{Token: "t", NewPassword: "StrongPassword123!", Now: time.Now()})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestExecute_HashFails(t *testing.T) {
	h := New(usecase.Deps{
		PasswordValidator: &passwordValidatorStub{},
		AuthTokens: &authTokenRepoStub{
			token: &domain.AuthToken{
				ID: "t1", UserID: "u1",
				ExpiresAt: time.Now().Add(1 * time.Hour),
			},
		},
		TokenHasher:   tokenHasherStub{},
		Hasher:        &hasherStub{err: errors.New("hash fail")},
		UsersWrite:    &userWriteStub{},
		SessionsWrite: &sessionsWriteStub{},
		Logger:        testLogger,
	})
	err := h.Execute(context.Background(), Command{Token: "t", NewPassword: "StrongPassword123!", Now: time.Now()})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestExecute_Success(t *testing.T) {
	h := New(usecase.Deps{
		PasswordValidator: &passwordValidatorStub{},
		AuthTokens: &authTokenRepoStub{
			token: &domain.AuthToken{
				ID: "t1", UserID: "u1",
				ExpiresAt: time.Now().Add(1 * time.Hour),
			},
		},
		TokenHasher:   tokenHasherStub{},
		Hasher:        &hasherStub{hash: "hashed-pw"},
		UsersWrite:    &userWriteStub{},
		SessionsWrite: &sessionsWriteStub{},
		Logger:        testLogger,
	})
	err := h.Execute(context.Background(), Command{Token: "t", NewPassword: "StrongPassword123!", Now: time.Now()})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
