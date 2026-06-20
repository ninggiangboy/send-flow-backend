package verification

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/shared"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
)

func verificationTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type verificationUsersReadStub struct {
	findByEmail func(context.Context, string) (*domain.User, error)
	findByID    func(context.Context, string) (*domain.User, error)
}

func (s *verificationUsersReadStub) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	if s.findByEmail != nil {
		return s.findByEmail(ctx, email)
	}
	return nil, domain.ErrNotFound
}
func (s *verificationUsersReadStub) FindByID(ctx context.Context, userID string) (*domain.User, error) {
	if s.findByID != nil {
		return s.findByID(ctx, userID)
	}
	return nil, domain.ErrNotFound
}

type verificationAuthTokenRepoStub struct {
	createErr error
	token     *domain.AuthToken
	err       error
}

func (s *verificationAuthTokenRepoStub) Create(context.Context, domain.AuthToken) error {
	return s.createErr
}
func (s *verificationAuthTokenRepoStub) FindByHash(context.Context, string, string) (*domain.AuthToken, error) {
	return s.token, s.err
}
func (s *verificationAuthTokenRepoStub) Consume(context.Context, string, time.Time) error { return nil }
func (s *verificationAuthTokenRepoStub) DeleteByUserAndPurpose(context.Context, string, string) error {
	return nil
}

type verificationTokenHasherStub struct{}

func (verificationTokenHasherStub) HashToken(token string) string { return "hashed-" + token }

type verificationIDGenStub struct{}

func (verificationIDGenStub) New() (string, error) { return "id1", nil }

type verificationTokenGenStub struct{}

func (verificationTokenGenStub) RandomToken(int) (string, error) { return "random-token", nil }

type verificationMailerStub struct {
	err error
}

func (s *verificationMailerStub) Send(context.Context, []string, string, string, string) error {
	return s.err
}

func TestRequestEmailHandlerExecuteUserNotFound(t *testing.T) {
	h := NewRequestEmailHandler(RequestEmailOptions{
		UsersRead: &verificationUsersReadStub{findByID: func(context.Context, string) (*domain.User, error) {
			return nil, domain.ErrNotFound
		}},
		Logger: verificationTestLogger(),
	})
	err := h.Execute(context.Background(), "u1", time.Now())
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestRequestEmailHandlerExecuteSuccess(t *testing.T) {
	authTokenSvc := shared.NewAuthTokenService(&verificationAuthTokenRepoStub{}, verificationTokenGenStub{}, verificationIDGenStub{}, verificationTokenHasherStub{})
	notifSvc := shared.NewNotificationService(&verificationMailerStub{}, "")
	h := NewRequestEmailHandler(RequestEmailOptions{
		UsersRead: &verificationUsersReadStub{findByID: func(context.Context, string) (*domain.User, error) {
			return &domain.User{ID: "u1", Email: "a@example.com"}, nil
		}},
		AuthTokens:      authTokenSvc,
		Notifications:   notifSvc,
		VerificationTTL: time.Minute,
		Logger:          verificationTestLogger(),
	})

	if err := h.Execute(context.Background(), "u1", time.Now()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
