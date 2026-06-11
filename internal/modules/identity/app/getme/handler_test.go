package getme

import (
	"context"
	"log/slog"
	"testing"

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

func TestExecute_NotFound(t *testing.T) {
	h := New(usecase.Deps{
		UsersRead: &usersReadStub{err: domain.ErrNotFound},
		Logger:    testLogger,
	})
	user, err := h.Execute(context.Background(), "u1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if user != nil {
		t.Fatal("expected nil user, got non-nil")
	}
}

func TestExecute_Success(t *testing.T) {
	u := &domain.User{ID: "u1", Email: "a@example.com"}
	h := New(usecase.Deps{
		UsersRead: &usersReadStub{user: u},
		Logger:    testLogger,
	})
	user, err := h.Execute(context.Background(), "u1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user == nil {
		t.Fatal("expected user, got nil")
	}
	if user.ID != "u1" {
		t.Fatalf("expected ID u1, got %s", user.ID)
	}
}
