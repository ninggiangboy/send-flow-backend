package session

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
)

func sessionTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type sessionWriteStub struct {
	revoked  bool
	findByID func(ctx context.Context, sessionID string) (*domain.Session, error)
}

func (s *sessionWriteStub) Create(context.Context, domain.Session) error { return nil }
func (s *sessionWriteStub) RevokeByID(context.Context, string, time.Time) error {
	s.revoked = true
	return nil
}
func (s *sessionWriteStub) RevokeByUser(context.Context, string, time.Time) error { return nil }
func (s *sessionWriteStub) RotateTokens(context.Context, string, string, string, time.Time, time.Time) error {
	return nil
}
func (s *sessionWriteStub) FindByID(ctx context.Context, sessionID string) (*domain.Session, error) {
	return s.findByID(ctx, sessionID)
}
func (s *sessionWriteStub) FindByAccessJTI(context.Context, string) (*domain.Session, error) {
	return nil, nil
}
func (s *sessionWriteStub) ListByUser(context.Context, string, time.Time) ([]domain.Session, error) {
	return nil, nil
}

type sessionRefreshStoreStub struct {
	deleted string
}

func (s *sessionRefreshStoreStub) Save(context.Context, string, string, time.Duration) error {
	return nil
}
func (s *sessionRefreshStoreStub) Find(context.Context, string) (string, error) { return "", nil }
func (s *sessionRefreshStoreStub) Delete(_ context.Context, refreshJTI string) error {
	s.deleted = refreshJTI
	return nil
}
func (s *sessionRefreshStoreStub) Replace(context.Context, string, string, string, time.Duration) error {
	return nil
}

func TestRevokeSessionHandlerExecuteRevokesOwnedSession(t *testing.T) {
	refresh := &sessionRefreshStoreStub{}
	write := &sessionWriteStub{findByID: func(context.Context, string) (*domain.Session, error) {
		return &domain.Session{ID: "s1", UserID: "u1", RefreshJTI: "r1", ExpiresAt: time.Now().Add(time.Hour)}, nil
	}}
	h := NewRevokeSessionHandler(Options{SessionsWrite: write, RefreshStore: refresh, Logger: sessionTestLogger()})

	if err := h.Execute(context.Background(), "s1", "u1", time.Now()); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !write.revoked || refresh.deleted != "r1" {
		t.Fatalf("unexpected side effects: revoked=%v deleted=%s", write.revoked, refresh.deleted)
	}
}

func TestRevokeSessionHandlerExecuteRejectsNonOwner(t *testing.T) {
	h := NewRevokeSessionHandler(Options{
		SessionsWrite: &sessionWriteStub{findByID: func(context.Context, string) (*domain.Session, error) {
			return &domain.Session{ID: "s1", UserID: "u1", RefreshJTI: "r1", ExpiresAt: time.Now().Add(time.Hour)}, nil
		}},
		RefreshStore: &sessionRefreshStoreStub{},
		Logger:       sessionTestLogger(),
	})

	err := h.Execute(context.Background(), "s1", "u2", time.Now())
	if !errors.Is(err, domain.ErrSessionNotOwned) {
		t.Fatalf("expected ErrSessionNotOwned, got %v", err)
	}
}
