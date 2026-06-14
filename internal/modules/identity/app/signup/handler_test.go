package signup

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/usecase"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/ports"
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
	h := New(Options{
		UsersRead:         &userReadStub{user: &domain.User{ID: "u1"}},
		UsersWrite:        &userWriteStub{},
		Hasher:            &hasherStub{hash: "h"},
		PasswordValidator: noopPasswordValidator{},
		Logger:            testLogger,
		UnitOfWork:        noopTx{},
		AuthTokens:        usecase.NewAuthTokenService(nil, nil, nil, nil),
	})

	_, err := h.Execute(context.Background(), Command{Email: "a@example.com", Password: "StrongPassword123!", Now: time.Now().UTC()})
	if !errors.Is(err, domain.ErrEmailAlreadyExists) {
		t.Fatalf("expected ErrEmailAlreadyExists, got %v", err)
	}
}

type tokenMgrStub struct {
	issue func(userID, sessionID string, now time.Time) (ports.TokenPair, string, string, error)
}

func (s *tokenMgrStub) Issue(userID, sessionID string, now time.Time) (ports.TokenPair, string, string, error) {
	return s.issue(userID, sessionID, now)
}
func (s *tokenMgrStub) ParseAccess(string) (*ports.AccessClaims, error)  { return nil, nil }
func (s *tokenMgrStub) ParseRefresh(string) (*ports.AccessClaims, error) { return nil, nil }

type sessWriteStub struct {
	create func(ctx context.Context, session domain.Session) error
}

func (s *sessWriteStub) Create(ctx context.Context, session domain.Session) error {
	return s.create(ctx, session)
}
func (s *sessWriteStub) RevokeByID(context.Context, string, time.Time) error   { return nil }
func (s *sessWriteStub) RevokeByUser(context.Context, string, time.Time) error { return nil }
func (s *sessWriteStub) RotateTokens(context.Context, string, string, string, time.Time, time.Time) error {
	return nil
}

type refreshStoreStub struct {
	save func(ctx context.Context, refreshJTI, sessionID string, ttl time.Duration) error
}

func (s *refreshStoreStub) Save(ctx context.Context, refreshJTI, sessionID string, ttl time.Duration) error {
	return s.save(ctx, refreshJTI, sessionID, ttl)
}
func (s *refreshStoreStub) Find(context.Context, string) (string, error) { return "", nil }
func (s *refreshStoreStub) Delete(context.Context, string) error         { return nil }
func (s *refreshStoreStub) Replace(context.Context, string, string, string, time.Duration) error {
	return nil
}

func TestExecuteSuccess(t *testing.T) {
	var called bool
	sessionIDGen := idGenStub{id: "sess-1"}
	tokenMgr := &tokenMgrStub{
		issue: func(_, _ string, now time.Time) (ports.TokenPair, string, string, error) {
			return ports.TokenPair{AccessToken: "at", RefreshToken: "rt", AccessExpiresAt: now.Add(time.Hour), RefreshExpiresAt: now.Add(24 * time.Hour)}, "access-jti", "refresh-jti", nil
		},
	}
	sessWrite := &sessWriteStub{
		create: func(_ context.Context, _ domain.Session) error {
			called = true
			return nil
		},
	}
	refreshStore := &refreshStoreStub{
		save: func(_ context.Context, _, _ string, _ time.Duration) error {
			return nil
		},
	}
	sessionFactory := usecase.NewSessionFactory(&sessionIDGen, tokenMgr, sessWrite, refreshStore, testLogger)
	h := New(Options{
		UsersRead:         &userReadStub{err: domain.ErrNotFound},
		IdGen:             idGenStub{id: "u1"},
		UsersWrite:        &userWriteStub{},
		Hasher:            &hasherStub{hash: "hashed"},
		PasswordValidator: noopPasswordValidator{},
		Logger:            testLogger,
		UnitOfWork:        noopTx{},
		SessionFactory:    sessionFactory,
		AuthTokens:        usecase.NewAuthTokenService(nil, nil, nil, nil),
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
