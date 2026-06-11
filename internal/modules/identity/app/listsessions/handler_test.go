package listsessions

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

type sessionsReadStub struct {
	sessions []domain.Session
	err      error
}

func (s *sessionsReadStub) FindByID(ctx context.Context, sessionID string) (*domain.Session, error) {
	return nil, nil
}
func (s *sessionsReadStub) FindByAccessJTI(ctx context.Context, jti string) (*domain.Session, error) {
	return nil, nil
}
func (s *sessionsReadStub) ListByUser(ctx context.Context, userID string, now time.Time) ([]domain.Session, error) {
	return s.sessions, s.err
}

func TestExecute_ListFails(t *testing.T) {
	h := New(usecase.Deps{
		SessionsRead: &sessionsReadStub{err: errors.New("db error")},
		Logger:       testLogger,
	})
	sessions, err := h.Execute(context.Background(), "u1", time.Now())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if sessions != nil {
		t.Fatal("expected nil sessions, got non-nil")
	}
}

func TestExecute_EmptyList(t *testing.T) {
	h := New(usecase.Deps{
		SessionsRead: &sessionsReadStub{sessions: []domain.Session{}},
		Logger:       testLogger,
	})
	sessions, err := h.Execute(context.Background(), "u1", time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sessions) != 0 {
		t.Fatalf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestExecute_Success(t *testing.T) {
	sessions := []domain.Session{
		{ID: "s1", UserID: "u1"},
		{ID: "s2", UserID: "u1"},
	}
	h := New(usecase.Deps{
		SessionsRead: &sessionsReadStub{sessions: sessions},
		Logger:       testLogger,
	})
	result, err := h.Execute(context.Background(), "u1", time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(result))
	}
}
