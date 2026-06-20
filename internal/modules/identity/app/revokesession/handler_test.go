package revokesession

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
)

var testLogger = slog.Default()

type sessionReadStub struct {
	sess *domain.Session
}

func (s *sessionReadStub) FindByID(context.Context, string) (*domain.Session, error) {
	return s.sess, nil
}
func (s *sessionReadStub) FindByAccessJTI(context.Context, string) (*domain.Session, error) {
	return s.sess, nil
}
func (s *sessionReadStub) ListByUser(context.Context, string, time.Time) ([]domain.Session, error) {
	return nil, nil
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
	if s.findByID != nil {
		return s.findByID(ctx, sessionID)
	}
	return nil, nil
}
func (s *sessionWriteStub) FindByAccessJTI(context.Context, string) (*domain.Session, error) {
	return nil, nil
}
func (s *sessionWriteStub) ListByUser(context.Context, string, time.Time) ([]domain.Session, error) {
	return nil, nil
}

type refreshStoreStub struct {
	deleted string
}

func (s *refreshStoreStub) Save(context.Context, string, string, time.Duration) error { return nil }
func (s *refreshStoreStub) Find(context.Context, string) (string, error)              { return "", nil }
func (s *refreshStoreStub) Delete(_ context.Context, refreshJTI string) error {
	s.deleted = refreshJTI
	return nil
}
func (s *refreshStoreStub) Replace(context.Context, string, string, string, time.Duration) error {
	return nil
}

func TestExecuteRevokesAndDeletesRefreshToken(t *testing.T) {
	refresh := &refreshStoreStub{}
	write := &sessionWriteStub{findByID: func(_ context.Context, _ string) (*domain.Session, error) {
		return &domain.Session{ID: "s1", UserID: "u1", RefreshJTI: "r1", ExpiresAt: time.Now().UTC().Add(1 * time.Hour)}, nil
	}}
	h := New(Options{
		SessionsWrite: write,
		RefreshStore:  refresh,
		Logger:        testLogger,
	})
	if err := h.Execute(context.Background(), "s1", "u1", time.Now().UTC()); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !write.revoked || refresh.deleted != "r1" {
		t.Fatalf("unexpected side effects: revoked=%v deleted=%s", write.revoked, refresh.deleted)
	}
}

func TestExecuteAlreadyRevokedIsNoop(t *testing.T) {
	refresh := &refreshStoreStub{}
	revokedAt := time.Now().UTC().Add(-1 * time.Minute)
	write := &sessionWriteStub{findByID: func(_ context.Context, _ string) (*domain.Session, error) {
		return &domain.Session{ID: "s1", UserID: "u1", RefreshJTI: "r1", RevokedAt: &revokedAt, ExpiresAt: time.Now().UTC().Add(1 * time.Hour)}, nil
	}}
	h := New(Options{
		SessionsWrite: write,
		RefreshStore:  refresh,
		Logger:        testLogger,
	})
	if err := h.Execute(context.Background(), "s1", "u1", time.Now().UTC()); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if write.revoked {
		t.Fatal("expected no revoke call for already-revoked session")
	}
	if refresh.deleted != "" {
		t.Fatal("expected no refresh delete for already-revoked session")
	}
}

func TestExecuteRejectsNonOwnedSession(t *testing.T) {
	refresh := &refreshStoreStub{}
	write := &sessionWriteStub{findByID: func(_ context.Context, _ string) (*domain.Session, error) {
		return &domain.Session{ID: "s1", UserID: "u1", RefreshJTI: "r1", ExpiresAt: time.Now().UTC().Add(1 * time.Hour)}, nil
	}}
	h := New(Options{
		SessionsWrite: write,
		RefreshStore:  refresh,
		Logger:        testLogger,
	})
	err := h.Execute(context.Background(), "s1", "u2", time.Now().UTC())
	if err == nil {
		t.Fatal("expected error when user does not own session")
	}
	if !errors.Is(err, domain.ErrSessionNotOwned) {
		t.Fatalf("expected ErrSessionNotOwned, got: %v", err)
	}
}
