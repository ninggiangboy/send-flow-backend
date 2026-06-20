package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/shared"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/ports"
)

func TestLoginHandlerExecuteReturnsInvalidCredentials(t *testing.T) {
	h := NewLoginHandler(LoginOptions{
		UsersWrite: &authUserWriteStub{findByEmail: func(context.Context, string) (*domain.User, error) {
			return nil, errors.New("db error")
		}},
		Hasher: &authHasherStub{},
		Logger: testLogger(),
	})

	_, err := h.Execute(context.Background(), LoginCommand{Email: "a@example.com", Password: "pw", Now: time.Now()})
	if !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestLoginHandlerExecuteSuccess(t *testing.T) {
	now := time.Now().UTC()
	sessionFactory := shared.NewSessionFactory(authIDGenStub{id: "sess_1"}, &authTokenManagerStub{
		issue: func(_, _ string, now time.Time) (ports.TokenPair, string, string, error) {
			return ports.TokenPair{AccessToken: "at", RefreshToken: "rt", AccessExpiresAt: now.Add(time.Hour), RefreshExpiresAt: now.Add(24 * time.Hour)}, "ajti", "rjti", nil
		},
	}, &authSessionWriteStub{}, &authRefreshStoreStub{}, testLogger())

	h := NewLoginHandler(LoginOptions{
		UsersWrite: &authUserWriteStub{findByEmail: func(context.Context, string) (*domain.User, error) {
			return &domain.User{ID: "u1", Email: "a@example.com", HashedPassword: "hash"}, nil
		}},
		Hasher:         &authHasherStub{},
		SessionFactory: sessionFactory,
		Logger:         testLogger(),
	})

	result, err := h.Execute(context.Background(), LoginCommand{Email: "A@EXAMPLE.com", Password: "pw", Now: now})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.User.ID != "u1" || result.SessionContext == nil {
		t.Fatalf("unexpected result: %+v", result)
	}
}
