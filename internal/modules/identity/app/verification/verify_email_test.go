package verification

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/shared"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
)

type verificationUserWriteStub struct {
	markEmailVerifiedErr error
}

func (s *verificationUserWriteStub) MarkEmailVerified(context.Context, string, time.Time) error {
	return s.markEmailVerifiedErr
}
func (s *verificationUserWriteStub) Create(context.Context, domain.User) error { return nil }
func (s *verificationUserWriteStub) UpdatePassword(context.Context, string, string, time.Time) error {
	return nil
}
func (s *verificationUserWriteStub) SetMFAEnabledAt(context.Context, string, *time.Time, time.Time) error {
	return nil
}
func (s *verificationUserWriteStub) FindByEmail(context.Context, string) (*domain.User, error) {
	return nil, nil
}
func (s *verificationUserWriteStub) FindByID(context.Context, string) (*domain.User, error) {
	return nil, nil
}

func TestVerifyEmailHandlerExecuteInvalidToken(t *testing.T) {
	authTokenSvc := shared.NewAuthTokenService(
		&verificationAuthTokenRepoStub{err: domain.ErrNotFound},
		verificationTokenGenStub{},
		verificationIDGenStub{},
		verificationTokenHasherStub{},
	)
	h := NewVerifyEmailHandler(VerifyEmailOptions{
		AuthTokens: authTokenSvc,
		UsersWrite: &verificationUserWriteStub{},
		Logger:     verificationTestLogger(),
	})
	err := h.Execute(context.Background(), "token", time.Now())
	if !errors.Is(err, domain.ErrVerificationToken) {
		t.Fatalf("expected ErrVerificationToken, got %v", err)
	}
}

func TestVerifyEmailHandlerExecuteSuccess(t *testing.T) {
	authTokenSvc := shared.NewAuthTokenService(
		&verificationAuthTokenRepoStub{token: &domain.AuthToken{ID: "t1", UserID: "u1", ExpiresAt: time.Now().Add(time.Hour)}},
		verificationTokenGenStub{},
		verificationIDGenStub{},
		verificationTokenHasherStub{},
	)
	h := NewVerifyEmailHandler(VerifyEmailOptions{
		AuthTokens: authTokenSvc,
		UsersWrite: &verificationUserWriteStub{},
		Logger:     verificationTestLogger(),
	})
	if err := h.Execute(context.Background(), "token", time.Now()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
