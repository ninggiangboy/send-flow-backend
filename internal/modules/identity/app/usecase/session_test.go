package usecase

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

type sessionWriteRepoStub struct {
	created []domain.Session
	err     error
}

func (s *sessionWriteRepoStub) Create(_ context.Context, sess domain.Session) error {
	if s.err != nil {
		return s.err
	}
	s.created = append(s.created, sess)
	return nil
}

func (s *sessionWriteRepoStub) RevokeByID(context.Context, string, time.Time) error { return nil }
func (s *sessionWriteRepoStub) RevokeByUser(context.Context, string, time.Time) error {
	return nil
}
func (s *sessionWriteRepoStub) RotateTokens(context.Context, string, string, string, time.Time, time.Time) error {
	return nil
}
func (s *sessionWriteRepoStub) FindByID(context.Context, string) (*domain.Session, error) {
	return nil, nil
}
func (s *sessionWriteRepoStub) FindByAccessJTI(context.Context, string) (*domain.Session, error) {
	return nil, nil
}
func (s *sessionWriteRepoStub) ListByUser(context.Context, string, time.Time) ([]domain.Session, error) {
	return nil, nil
}

type refreshStoreStub struct {
	calls int
	err   error
}

func (s *refreshStoreStub) Save(context.Context, string, string, time.Duration) error {
	s.calls++
	return s.err
}
func (s *refreshStoreStub) Delete(context.Context, string) error { return nil }
func (s *refreshStoreStub) Find(context.Context, string) (string, error) {
	return "", nil
}
func (s *refreshStoreStub) Replace(context.Context, string, string, string, time.Duration) error {
	return nil
}

type idGenStub struct {
	id  string
	err error
}

func (s *idGenStub) New() (string, error) {
	return s.id, s.err
}

type tokenManagerStub struct {
	pair ports.TokenPair
	err  error
}

func (s *tokenManagerStub) Issue(userID, sessionID string, now time.Time) (ports.TokenPair, string, string, error) {
	if s.err != nil {
		return ports.TokenPair{}, "", "", s.err
	}
	return s.pair, "access-jti", "refresh-jti", nil
}
func (s *tokenManagerStub) ParseAccess(string, time.Time) (*ports.AccessClaims, error) {
	return nil, nil
}
func (s *tokenManagerStub) ParseRefresh(string, time.Time) (*ports.AccessClaims, error) {
	return nil, nil
}

func TestBuildNewSessionSuccess(t *testing.T) {
	now := time.Now().UTC()
	sf := NewSessionFactory(
		&idGenStub{id: "session-1"},
		&tokenManagerStub{pair: ports.TokenPair{AccessToken: "a", RefreshToken: "r", AccessExpiresAt: now.Add(10 * time.Minute), RefreshExpiresAt: now.Add(20 * time.Minute)}},
		&sessionWriteRepoStub{},
		&refreshStoreStub{},
		testLogger,
	)
	out, err := sf.NewSession(context.Background(), NewSessionInput{
		User:   domain.User{ID: "u1", Email: "a@example.com"},
		Method: "password",
		IP:     "127.0.0.1",
		UA:     "agent",
		Now:    now,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.User.ID != "u1" || out.Session.UserID != "u1" {
		t.Fatalf("unexpected session context: %+v", out)
	}
}

func TestBuildNewSessionFailsWhenTokenIssueFails(t *testing.T) {
	sf := NewSessionFactory(
		&idGenStub{id: "session-1"},
		&tokenManagerStub{pair: ports.TokenPair{}, err: errors.New("issue error")},
		&sessionWriteRepoStub{},
		&refreshStoreStub{},
		testLogger,
	)
	_, err := sf.NewSession(context.Background(), NewSessionInput{User: domain.User{ID: "u1"}, Now: time.Now().UTC()})
	if err == nil {
		t.Fatal("expected error")
	}
}
