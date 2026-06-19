package authenticate

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/ports"
)

var testLogger = slog.Default()

type tokenStub struct {
	claims *ports.AccessClaims
	err    error
}

func (s *tokenStub) Issue(string, string, time.Time) (ports.TokenPair, string, string, error) {
	return ports.TokenPair{}, "", "", nil
}
func (s *tokenStub) ParseAccess(string, time.Time) (*ports.AccessClaims, error) {
	return s.claims, s.err
}
func (s *tokenStub) ParseRefresh(string, time.Time) (*ports.AccessClaims, error) {
	return s.claims, s.err
}

type sessionsStub struct {
	session *domain.Session
	err     error
}

func (s *sessionsStub) FindByID(_ context.Context, sessionID string) (*domain.Session, error) {
	return s.session, s.err
}
func (s *sessionsStub) FindByAccessJTI(_ context.Context, jti string) (*domain.Session, error) {
	return nil, domain.ErrNotFound
}
func (s *sessionsStub) ListByUser(_ context.Context, userID string, now time.Time) ([]domain.Session, error) {
	return nil, nil
}

func TestExecuteUnauthorizedWhenTokenInvalid(t *testing.T) {
	sessions := &sessionsStub{session: &domain.Session{ID: "s1", UserID: "u1", ExpiresAt: time.Now().UTC().Add(1 * time.Hour)}}
	h := New(&tokenStub{err: errors.New("bad token")}, sessions, testLogger)
	_, _, err := h.Execute(context.Background(), "token", time.Now().UTC())
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("expected unauthorized, got %v", err)
	}
}

func TestExecuteSuccess(t *testing.T) {
	sessions := &sessionsStub{session: &domain.Session{ID: "s1", UserID: "u1", ExpiresAt: time.Now().UTC().Add(1 * time.Hour)}}
	h := New(&tokenStub{claims: &ports.AccessClaims{Subject: "u1", SessionID: "s1", JWTID: "j1", ExpiresAt: time.Now().UTC().Add(1 * time.Hour).Unix()}}, sessions, testLogger)
	sess, user, err := h.Execute(context.Background(), "token", time.Now().UTC())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sess.ID != "s1" || sess.UserID != "u1" || user.ID != "u1" {
		t.Fatalf("unexpected result: %v %v", sess, user)
	}
}

func TestExecuteUnauthorizedWhenClaimsIncomplete(t *testing.T) {
	sessions := &sessionsStub{session: &domain.Session{ID: "s1", UserID: "u1", ExpiresAt: time.Now().UTC().Add(1 * time.Hour)}}
	h := New(&tokenStub{claims: &ports.AccessClaims{Subject: "u1", JWTID: "j1"}}, sessions, testLogger)
	_, _, err := h.Execute(context.Background(), "token", time.Now().UTC())
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("expected unauthorized for incomplete claims, got %v", err)
	}
}

func TestExecuteUnauthorizedWhenSessionRevoked(t *testing.T) {
	now := time.Now().UTC()
	revokedAt := now
	sessions := &sessionsStub{session: &domain.Session{ID: "s1", UserID: "u1", ExpiresAt: now.Add(1 * time.Hour), RevokedAt: &revokedAt}}
	h := New(&tokenStub{claims: &ports.AccessClaims{Subject: "u1", SessionID: "s1", JWTID: "j1", ExpiresAt: now.Add(1 * time.Hour).Unix()}}, sessions, testLogger)
	_, _, err := h.Execute(context.Background(), "token", now)
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("expected unauthorized for revoked session, got %v", err)
	}
}

func TestExecuteUnauthorizedWhenSessionNotFound(t *testing.T) {
	sessions := &sessionsStub{err: domain.ErrNotFound}
	h := New(&tokenStub{claims: &ports.AccessClaims{Subject: "u1", SessionID: "s1", JWTID: "j1", ExpiresAt: time.Now().UTC().Add(1 * time.Hour).Unix()}}, sessions, testLogger)
	_, _, err := h.Execute(context.Background(), "token", time.Now().UTC())
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("expected unauthorized for missing session, got %v", err)
	}
}
