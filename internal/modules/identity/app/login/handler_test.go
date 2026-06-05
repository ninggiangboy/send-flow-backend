package login

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

type userReadStub struct {
	user *domain.User
	err  error
}

func (s *userReadStub) FindByEmail(context.Context, string) (*domain.User, error) {
	return s.user, s.err
}
func (s *userReadStub) FindByID(context.Context, string) (*domain.User, error) { return s.user, s.err }

type hasherStub struct{ err error }

func (s *hasherStub) Hash(string) (string, error)  { return "", nil }
func (s *hasherStub) Compare(string, string) error { return s.err }

func TestExecuteReturnsInvalidCredentials(t *testing.T) {
	h := New(usecase.Deps{
		UsersRead: &userReadStub{err: errors.New("db error")},
		Hasher:    &hasherStub{},
		Logger:    testLogger,
	}, func(context.Context, usecase.NewSessionInput) (*usecase.SessionContext, error) {
		return nil, nil
	})
	_, err := h.Execute(context.Background(), Command{Email: "a@example.com", Password: "pw", Now: time.Now().UTC()})
	if !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestExecuteSuccess(t *testing.T) {
	h := New(usecase.Deps{
		UsersRead: &userReadStub{user: &domain.User{ID: "u1", Email: "a@example.com", HashedPassword: "hash"}},
		Hasher:    &hasherStub{},
		Logger:    testLogger,
	}, func(_ context.Context, in usecase.NewSessionInput) (*usecase.SessionContext, error) {
		return &usecase.SessionContext{User: in.User}, nil
	})
	out, err := h.Execute(context.Background(), Command{Email: "A@EXAMPLE.com", Password: "pw", Now: time.Now().UTC()})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.User.ID != "u1" {
		t.Fatalf("unexpected user: %+v", out.User)
	}
}
