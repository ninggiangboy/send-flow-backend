package verifyemail

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

type tokenGenStub struct{}

func (tokenGenStub) RandomToken(n int) (string, error) { return "token", nil }

type idGenStub struct{}

func (idGenStub) New() (string, error) { return "id", nil }

type userWriteStub struct {
	markEmailVerifiedErr error
}

func (s *userWriteStub) MarkEmailVerified(ctx context.Context, userID string, at time.Time) error {
	return s.markEmailVerifiedErr
}
func (s *userWriteStub) Create(ctx context.Context, user domain.User) error { return nil }
func (s *userWriteStub) UpdatePassword(ctx context.Context, userID, hashedPassword string, at time.Time) error {
	return nil
}
func (s *userWriteStub) SetMFAEnabledAt(ctx context.Context, userID string, enabledAt *time.Time, at time.Time) error {
	return nil
}

func TestExecute_InvalidToken(t *testing.T) {
	authTokenSvc := usecase.NewAuthTokenService(&authTokenRepoStub{err: domain.ErrNotFound}, tokenGenStub{}, idGenStub{}, tokenHasherStub{})
	h := New(Options{
		AuthTokens: authTokenSvc,
		UsersWrite: &userWriteStub{},
		Logger:     testLogger,
	})
	err := h.Execute(context.Background(), "token", time.Now())
	if !errors.Is(err, domain.ErrVerificationToken) {
		t.Fatalf("expected ErrVerificationToken, got %v", err)
	}
}

func TestExecute_ExpiredToken(t *testing.T) {
	authTokenSvc := usecase.NewAuthTokenService(
		&authTokenRepoStub{
			token: &domain.AuthToken{
				ID:        "t1",
				ExpiresAt: time.Now().Add(-1 * time.Hour),
			},
		},
		tokenGenStub{}, idGenStub{}, tokenHasherStub{},
	)
	h := New(Options{
		AuthTokens: authTokenSvc,
		UsersWrite: &userWriteStub{},
		Logger:     testLogger,
	})
	err := h.Execute(context.Background(), "token", time.Now())
	if !errors.Is(err, domain.ErrVerificationToken) {
		t.Fatalf("expected ErrVerificationToken, got %v", err)
	}
}

func TestExecute_MarkEmailVerifiedFails(t *testing.T) {
	authTokenSvc := usecase.NewAuthTokenService(
		&authTokenRepoStub{
			token: &domain.AuthToken{
				ID:        "t1",
				UserID:    "u1",
				ExpiresAt: time.Now().Add(1 * time.Hour),
			},
		},
		tokenGenStub{}, idGenStub{}, tokenHasherStub{},
	)
	h := New(Options{
		AuthTokens: authTokenSvc,
		UsersWrite: &userWriteStub{markEmailVerifiedErr: errors.New("db error")},
		Logger:     testLogger,
	})
	err := h.Execute(context.Background(), "token", time.Now())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestExecute_Success(t *testing.T) {
	authTokenSvc := usecase.NewAuthTokenService(
		&authTokenRepoStub{
			token: &domain.AuthToken{
				ID:        "t1",
				UserID:    "u1",
				ExpiresAt: time.Now().Add(1 * time.Hour),
			},
		},
		tokenGenStub{}, idGenStub{}, tokenHasherStub{},
	)
	h := New(Options{
		AuthTokens: authTokenSvc,
		UsersWrite: &userWriteStub{},
		Logger:     testLogger,
	})
	err := h.Execute(context.Background(), "token", time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
