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

type authUnitOfWorkStub struct{}

func (authUnitOfWorkStub) WithinTx(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

type authOutboxStub struct{}

func (authOutboxStub) Save(context.Context, ports.OutboxEvent) error { return nil }

type authMailerStub struct{}

func (authMailerStub) Send(context.Context, []string, string, string, string) error { return nil }

func TestSignupHandlerExecuteReturnsDuplicateWhenEmailExists(t *testing.T) {
	h := NewSignupHandler(SignupOptions{
		UsersWrite: &authUserWriteStub{findByEmail: func(context.Context, string) (*domain.User, error) {
			return &domain.User{ID: "u1"}, nil
		}},
		Hasher:            &authHasherStub{hash: "hash"},
		PasswordValidator: authPasswordValidatorStub{},
		Logger:            testLogger(),
		UnitOfWork:        authUnitOfWorkStub{},
		AuthTokens:        shared.NewAuthTokenService(&authTokenRepoStub{}, authTokenGenStub{}, authIDGenStub{id: "tok"}, authTokenHasherStub{}),
	})

	_, err := h.Execute(context.Background(), SignupCommand{Email: "a@example.com", Password: "StrongPassword123!", Now: time.Now()})
	if !errors.Is(err, domain.ErrEmailAlreadyExists) {
		t.Fatalf("expected ErrEmailAlreadyExists, got %v", err)
	}
}

func TestSignupHandlerExecuteSuccess(t *testing.T) {
	var created bool
	sessionFactory := shared.NewSessionFactory(authIDGenStub{id: "sess_1"}, &authTokenManagerStub{
		issue: func(_, _ string, now time.Time) (ports.TokenPair, string, string, error) {
			return ports.TokenPair{AccessToken: "at", RefreshToken: "rt", AccessExpiresAt: now.Add(time.Hour), RefreshExpiresAt: now.Add(24 * time.Hour)}, "ajti", "rjti", nil
		},
	}, &authSessionWriteStub{
		create: func(context.Context, domain.Session) error { created = true; return nil },
	}, &authRefreshStoreStub{}, testLogger())

	h := NewSignupHandler(SignupOptions{
		UsersWrite: &authUserWriteStub{
			findByEmail: func(context.Context, string) (*domain.User, error) { return nil, domain.ErrNotFound },
			create:      func(context.Context, domain.User) error { return nil },
		},
		IdGen:             authIDGenStub{id: "u1"},
		Hasher:            &authHasherStub{hash: "hashed"},
		PasswordValidator: authPasswordValidatorStub{},
		OutboxWriter:      authOutboxStub{},
		UnitOfWork:        authUnitOfWorkStub{},
		SessionFactory:    sessionFactory,
		AuthTokens:        shared.NewAuthTokenService(&authTokenRepoStub{}, authTokenGenStub{}, authIDGenStub{id: "tok"}, authTokenHasherStub{}),
		Notifications:     shared.NewNotificationService(authMailerStub{}, "http://localhost:3000"),
		Logger:            testLogger(),
	})

	result, err := h.Execute(context.Background(), SignupCommand{Email: " A@EXAMPLE.com ", Password: "StrongPassword123!", Now: time.Now()})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !created || result.User.Email != "a@example.com" {
		t.Fatalf("unexpected signup result: %+v created=%v", result, created)
	}
}
