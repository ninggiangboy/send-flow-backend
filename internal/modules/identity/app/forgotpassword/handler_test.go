package forgotpassword

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

type usersReadStub struct {
	user *domain.User
	err  error
}

func (s *usersReadStub) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	return s.user, s.err
}
func (s *usersReadStub) FindByID(ctx context.Context, userID string) (*domain.User, error) {
	return s.user, s.err
}

type authTokenRepoStub struct {
	createErr error
}

func (s *authTokenRepoStub) Create(ctx context.Context, token domain.AuthToken) error {
	return s.createErr
}
func (s *authTokenRepoStub) FindByHash(ctx context.Context, purpose, tokenHash string) (*domain.AuthToken, error) {
	return nil, nil
}
func (s *authTokenRepoStub) Consume(ctx context.Context, tokenID string, at time.Time) error {
	return nil
}
func (s *authTokenRepoStub) DeleteByUserAndPurpose(ctx context.Context, userID, purpose string) error {
	return nil
}

type tokenHasherStub struct{}

func (tokenHasherStub) HashToken(token string) string { return "hashed-" + token }

type idGenStub struct{}

func (idGenStub) New() (string, error) { return "id1", nil }

type tokenGenStub struct{}

func (tokenGenStub) RandomToken(n int) (string, error) { return "random-token", nil }

type mailerStub struct {
	err error
}

func (s *mailerStub) Send(ctx context.Context, to []string, subject, text, html string) error {
	return s.err
}

func TestExecute_InvalidEmailFormat(t *testing.T) {
	h := New(usecase.Deps{
		Logger: testLogger,
	})
	err := h.Execute(context.Background(), "not-an-email", time.Now())
	if err != nil {
		t.Fatalf("expected nil for invalid email, got %v", err)
	}
}

func TestExecute_UserNotFound(t *testing.T) {
	h := New(usecase.Deps{
		UsersRead: &usersReadStub{err: domain.ErrNotFound},
		Logger:    testLogger,
	})
	err := h.Execute(context.Background(), "a@example.com", time.Now())
	if err != nil {
		t.Fatalf("expected nil for not-found user, got %v", err)
	}
}

func TestExecute_UserLookupFails(t *testing.T) {
	h := New(usecase.Deps{
		UsersRead: &usersReadStub{err: errors.New("db error")},
		Logger:    testLogger,
	})
	err := h.Execute(context.Background(), "a@example.com", time.Now())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestExecute_TokenCreationFails(t *testing.T) {
	h := New(usecase.Deps{
		UsersRead:        &usersReadStub{user: &domain.User{ID: "u1", Email: "a@example.com"}},
		AuthTokens:       &authTokenRepoStub{createErr: errors.New("token err")},
		TokenHasher:      tokenHasherStub{},
		IDGen:            idGenStub{},
		TokenGen:         tokenGenStub{},
		PasswordResetTTL: time.Minute,
		Logger:           testLogger,
	})
	err := h.Execute(context.Background(), "a@example.com", time.Now())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestExecute_SendEmailFails(t *testing.T) {
	h := New(usecase.Deps{
		UsersRead:        &usersReadStub{user: &domain.User{ID: "u1", Email: "a@example.com"}},
		AuthTokens:       &authTokenRepoStub{},
		TokenHasher:      tokenHasherStub{},
		IDGen:            idGenStub{},
		TokenGen:         tokenGenStub{},
		PasswordResetTTL: time.Minute,
		MailSender:       &mailerStub{err: errors.New("send error")},
		Logger:           testLogger,
	})
	err := h.Execute(context.Background(), "a@example.com", time.Now())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestExecute_Success(t *testing.T) {
	h := New(usecase.Deps{
		UsersRead:        &usersReadStub{user: &domain.User{ID: "u1", Email: "a@example.com"}},
		AuthTokens:       &authTokenRepoStub{},
		TokenHasher:      tokenHasherStub{},
		IDGen:            idGenStub{},
		TokenGen:         tokenGenStub{},
		PasswordResetTTL: time.Minute,
		MailSender:       &mailerStub{},
		Logger:           testLogger,
	})
	err := h.Execute(context.Background(), "a@example.com", time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
