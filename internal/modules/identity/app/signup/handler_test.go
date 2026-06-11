package signup

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/usecase"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
)

type noopTx struct{}

func (noopTx) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error { return fn(ctx) }

var testLogger = slog.Default()

type userReadStub struct {
	user *domain.User
	err  error
}

func (s *userReadStub) FindByEmail(context.Context, string) (*domain.User, error) {
	return s.user, s.err
}
func (s *userReadStub) FindByID(context.Context, string) (*domain.User, error) { return s.user, s.err }

type userWriteStub struct {
	last *domain.User
	err  error
}

func (s *userWriteStub) Create(_ context.Context, u domain.User) error {
	if s.err != nil {
		return s.err
	}
	s.last = &u
	return nil
}
func (s *userWriteStub) UpdatePassword(context.Context, string, string, time.Time) error {
	return nil
}
func (s *userWriteStub) MarkEmailVerified(context.Context, string, time.Time) error { return nil }
func (s *userWriteStub) SetMFAEnabledAt(context.Context, string, *time.Time, time.Time) error {
	return nil
}

type hasherStub struct {
	hash string
	err  error
}

func (s *hasherStub) Hash(string) (string, error)  { return s.hash, s.err }
func (s *hasherStub) Compare(string, string) error { return nil }

type noopPasswordValidator struct{}

func (noopPasswordValidator) Validate(string) error { return nil }

type idGenStub struct {
	id string
}

func (s idGenStub) New() (string, error) { return s.id, nil }

func TestExecuteReturnsDuplicateWhenEmailExists(t *testing.T) {
	h := New(usecase.Deps{
		UsersRead:         &userReadStub{user: &domain.User{ID: "u1"}},
		UsersWrite:        &userWriteStub{},
		Hasher:            &hasherStub{hash: "h"},
		PasswordValidator: noopPasswordValidator{},
		Logger:            testLogger,
		UnitOfWork:        noopTx{},
	}, func(context.Context, usecase.NewSessionInput) (*usecase.SessionContext, error) {
		return nil, nil
	})

	_, err := h.Execute(context.Background(), Command{Email: "a@example.com", Password: "StrongPassword123!", Now: time.Now().UTC()})
	if !errors.Is(err, domain.ErrEmailAlreadyExists) {
		t.Fatalf("expected ErrEmailAlreadyExists, got %v", err)
	}
}

func TestExecuteSuccess(t *testing.T) {
	var called bool
	h := New(usecase.Deps{
		UsersRead:         &userReadStub{err: domain.ErrNotFound},
		IDGen:             idGenStub{id: "u1"},
		UsersWrite:        &userWriteStub{},
		Hasher:            &hasherStub{hash: "hashed"},
		PasswordValidator: noopPasswordValidator{},
		Logger:            testLogger,
		UnitOfWork:        noopTx{},
	}, func(_ context.Context, in usecase.NewSessionInput) (*usecase.SessionContext, error) {
		called = true
		return &usecase.SessionContext{User: in.User}, nil
	})

	res, err := h.Execute(context.Background(), Command{Email: "  A@EXAMPLE.com ", Password: "StrongPassword123!", Now: time.Now().UTC()})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("expected newSession to be called")
	}
	if res.User.Email != "a@example.com" {
		t.Fatalf("expected normalized email, got %s", res.User.Email)
	}
}
