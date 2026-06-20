package refresh

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

type tokenManagerStub struct {
	parseRefresh func(token string, now time.Time) (*ports.AccessClaims, error)
	issue        func(userID, sessionID string, now time.Time) (ports.TokenPair, string, string, error)
}

func (s *tokenManagerStub) ParseRefresh(token string, now time.Time) (*ports.AccessClaims, error) {
	return s.parseRefresh(token, now)
}
func (s *tokenManagerStub) ParseAccess(token string, now time.Time) (*ports.AccessClaims, error) {
	return nil, nil
}
func (s *tokenManagerStub) Issue(userID, sessionID string, now time.Time) (ports.TokenPair, string, string, error) {
	return s.issue(userID, sessionID, now)
}

type refreshStoreStub struct {
	find    func(ctx context.Context, refreshJTI string) (string, error)
	replace func(ctx context.Context, oldRefreshJTI, newRefreshJTI, sessionID string, ttl time.Duration) error
}

func (s *refreshStoreStub) Save(context.Context, string, string, time.Duration) error {
	return nil
}
func (s *refreshStoreStub) Find(ctx context.Context, refreshJTI string) (string, error) {
	return s.find(ctx, refreshJTI)
}
func (s *refreshStoreStub) Delete(context.Context, string) error { return nil }
func (s *refreshStoreStub) Replace(ctx context.Context, oldRefreshJTI, newRefreshJTI, sessionID string, ttl time.Duration) error {
	return s.replace(ctx, oldRefreshJTI, newRefreshJTI, sessionID, ttl)
}

type usersWriteStub struct {
	findByID func(ctx context.Context, userID string) (*domain.User, error)
}

func (s *usersWriteStub) Create(context.Context, domain.User) error                       { return nil }
func (s *usersWriteStub) UpdatePassword(context.Context, string, string, time.Time) error { return nil }
func (s *usersWriteStub) MarkEmailVerified(context.Context, string, time.Time) error      { return nil }
func (s *usersWriteStub) SetMFAEnabledAt(context.Context, string, *time.Time, time.Time) error {
	return nil
}
func (s *usersWriteStub) FindByEmail(context.Context, string) (*domain.User, error) { return nil, nil }
func (s *usersWriteStub) FindByID(ctx context.Context, userID string) (*domain.User, error) {
	if s.findByID != nil {
		return s.findByID(ctx, userID)
	}
	return nil, nil
}

type sessionsWriteStub struct {
	rotateTokens func(ctx context.Context, sessionID, accessJTI, refreshJTI string, expiresAt, now time.Time) error
	findByID     func(ctx context.Context, sessionID string) (*domain.Session, error)
}

func (s *sessionsWriteStub) Create(ctx context.Context, session domain.Session) error {
	return nil
}
func (s *sessionsWriteStub) RevokeByID(ctx context.Context, sessionID string, now time.Time) error {
	return nil
}
func (s *sessionsWriteStub) RevokeByUser(ctx context.Context, userID string, now time.Time) error {
	return nil
}
func (s *sessionsWriteStub) RotateTokens(ctx context.Context, sessionID, accessJTI, refreshJTI string, expiresAt, now time.Time) error {
	return s.rotateTokens(ctx, sessionID, accessJTI, refreshJTI, expiresAt, now)
}
func (s *sessionsWriteStub) FindByID(ctx context.Context, sessionID string) (*domain.Session, error) {
	if s.findByID != nil {
		return s.findByID(ctx, sessionID)
	}
	return nil, nil
}
func (s *sessionsWriteStub) FindByAccessJTI(context.Context, string) (*domain.Session, error) {
	return nil, nil
}
func (s *sessionsWriteStub) ListByUser(context.Context, string, time.Time) ([]domain.Session, error) {
	return nil, nil
}

func TestRefreshSuccess(t *testing.T) {
	now := time.Now().UTC()
	sess := domain.Session{
		ID:         "sess-1",
		UserID:     "u1",
		AccessJTI:  "old-access-jti",
		RefreshJTI: "old-refresh-jti",
		ExpiresAt:  now.Add(24 * time.Hour),
	}
	h := New(Options{
		Logger: testLogger,
		Tokens: &tokenManagerStub{
			parseRefresh: func(_ string, _ time.Time) (*ports.AccessClaims, error) {
				return &ports.AccessClaims{JWTID: "old-refresh-jti", SessionID: "sess-1"}, nil
			},
			issue: func(_, _ string, _ time.Time) (ports.TokenPair, string, string, error) {
				return ports.TokenPair{
					AccessToken: "new-access", RefreshToken: "new-refresh",
					AccessExpiresAt: now.Add(10 * time.Minute), RefreshExpiresAt: now.Add(20 * time.Minute),
				}, "new-access-jti", "new-refresh-jti", nil
			},
		},
		RefreshStore: &refreshStoreStub{
			find: func(_ context.Context, _ string) (string, error) {
				return "sess-1", nil
			},
			replace: func(_ context.Context, _, _, _ string, _ time.Duration) error {
				return nil
			},
		},
		SessionsWrite: &sessionsWriteStub{
			findByID: func(_ context.Context, _ string) (*domain.Session, error) {
				cp := sess
				return &cp, nil
			},
			rotateTokens: func(_ context.Context, _, _, _ string, _, _ time.Time) error {
				return nil
			},
		},
		UsersWrite: &usersWriteStub{
			findByID: func(_ context.Context, _ string) (*domain.User, error) {
				return &domain.User{ID: "u1", Email: "a@example.com"}, nil
			},
		},
	})
	result, err := h.Execute(context.Background(), Command{RefreshToken: "valid-token", Now: now})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.User.ID != "u1" || result.Session.ID != "sess-1" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if result.Tokens.AccessToken != "new-access" {
		t.Fatalf("unexpected access token: %s", result.Tokens.AccessToken)
	}
}

func TestRefresh_InvalidToken(t *testing.T) {
	h := New(Options{
		Logger: testLogger,
		Tokens: &tokenManagerStub{
			parseRefresh: func(_ string, _ time.Time) (*ports.AccessClaims, error) {
				return nil, errors.New("parse error")
			},
		},
	})
	_, err := h.Execute(context.Background(), Command{RefreshToken: "invalid-token"})
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got: %v", err)
	}
}

func TestRefresh_JTINotFound(t *testing.T) {
	h := New(Options{
		Logger: testLogger,
		Tokens: &tokenManagerStub{
			parseRefresh: func(_ string, _ time.Time) (*ports.AccessClaims, error) {
				return &ports.AccessClaims{JWTID: "jti-1", SessionID: "sess-1"}, nil
			},
		},
		RefreshStore: &refreshStoreStub{
			find: func(_ context.Context, _ string) (string, error) {
				return "", ports.ErrCacheMiss
			},
		},
	})
	_, err := h.Execute(context.Background(), Command{RefreshToken: "token"})
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got: %v", err)
	}
}

func TestRefresh_RefreshStoreError(t *testing.T) {
	expectedErr := errors.New("cache error")
	h := New(Options{
		Logger: testLogger,
		Tokens: &tokenManagerStub{
			parseRefresh: func(_ string, _ time.Time) (*ports.AccessClaims, error) {
				return &ports.AccessClaims{JWTID: "jti-1", SessionID: "sess-1"}, nil
			},
		},
		RefreshStore: &refreshStoreStub{
			find: func(_ context.Context, _ string) (string, error) {
				return "", expectedErr
			},
		},
	})
	_, err := h.Execute(context.Background(), Command{RefreshToken: "token"})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got: %v", expectedErr, err)
	}
}

func TestRefresh_SessionMismatch(t *testing.T) {
	h := New(Options{
		Logger: testLogger,
		Tokens: &tokenManagerStub{
			parseRefresh: func(_ string, _ time.Time) (*ports.AccessClaims, error) {
				return &ports.AccessClaims{JWTID: "jti-1", SessionID: "sess-1"}, nil
			},
		},
		RefreshStore: &refreshStoreStub{
			find: func(_ context.Context, _ string) (string, error) {
				return "sess-2", nil
			},
		},
	})
	_, err := h.Execute(context.Background(), Command{RefreshToken: "token"})
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got: %v", err)
	}
}

func TestRefresh_SessionInactive(t *testing.T) {
	now := time.Now().UTC()
	h := New(Options{
		Logger: testLogger,
		Tokens: &tokenManagerStub{
			parseRefresh: func(_ string, _ time.Time) (*ports.AccessClaims, error) {
				return &ports.AccessClaims{JWTID: "jti-1", SessionID: "sess-1"}, nil
			},
		},
		RefreshStore: &refreshStoreStub{
			find: func(_ context.Context, _ string) (string, error) {
				return "sess-1", nil
			},
		},
		SessionsWrite: &sessionsWriteStub{
			findByID: func(_ context.Context, _ string) (*domain.Session, error) {
				return &domain.Session{
					ID: "sess-1", UserID: "u1", RefreshJTI: "jti-1",
					RevokedAt: &now, // revoked
				}, nil
			},
		},
	})
	_, err := h.Execute(context.Background(), Command{RefreshToken: "token", Now: now})
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got: %v", err)
	}
}

func TestRefresh_SessionExpired(t *testing.T) {
	now := time.Now().UTC()
	h := New(Options{
		Logger: testLogger,
		Tokens: &tokenManagerStub{
			parseRefresh: func(_ string, _ time.Time) (*ports.AccessClaims, error) {
				return &ports.AccessClaims{JWTID: "jti-1", SessionID: "sess-1"}, nil
			},
		},
		RefreshStore: &refreshStoreStub{
			find: func(_ context.Context, _ string) (string, error) {
				return "sess-1", nil
			},
		},
		SessionsWrite: &sessionsWriteStub{
			findByID: func(_ context.Context, _ string) (*domain.Session, error) {
				// expired session
				return &domain.Session{
					ID: "sess-1", UserID: "u1", RefreshJTI: "jti-1",
					ExpiresAt: now.Add(-1 * time.Hour),
				}, nil
			},
		},
	})
	_, err := h.Execute(context.Background(), Command{RefreshToken: "token", Now: now})
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got: %v", err)
	}
}

func TestRefresh_SessionJTIMismatch(t *testing.T) {
	now := time.Now().UTC()
	h := New(Options{
		Logger: testLogger,
		Tokens: &tokenManagerStub{
			parseRefresh: func(_ string, _ time.Time) (*ports.AccessClaims, error) {
				return &ports.AccessClaims{JWTID: "jti-1", SessionID: "sess-1"}, nil
			},
		},
		RefreshStore: &refreshStoreStub{
			find: func(_ context.Context, _ string) (string, error) {
				return "sess-1", nil
			},
		},
		SessionsWrite: &sessionsWriteStub{
			findByID: func(_ context.Context, _ string) (*domain.Session, error) {
				return &domain.Session{
					ID: "sess-1", UserID: "u1", RefreshJTI: "different-jti",
					ExpiresAt: now.Add(24 * time.Hour),
				}, nil
			},
		},
	})
	_, err := h.Execute(context.Background(), Command{RefreshToken: "token", Now: now})
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got: %v", err)
	}
}

func TestRefresh_UserNotFound(t *testing.T) {
	now := time.Now().UTC()
	h := New(Options{
		Logger: testLogger,
		Tokens: &tokenManagerStub{
			parseRefresh: func(_ string, _ time.Time) (*ports.AccessClaims, error) {
				return &ports.AccessClaims{JWTID: "jti-1", SessionID: "sess-1"}, nil
			},
		},
		RefreshStore: &refreshStoreStub{
			find: func(_ context.Context, _ string) (string, error) {
				return "sess-1", nil
			},
		},
		SessionsWrite: &sessionsWriteStub{
			findByID: func(_ context.Context, _ string) (*domain.Session, error) {
				return &domain.Session{ID: "sess-1", UserID: "u1", RefreshJTI: "jti-1", ExpiresAt: now.Add(24 * time.Hour)}, nil
			},
		},
		UsersWrite: &usersWriteStub{
			findByID: func(_ context.Context, _ string) (*domain.User, error) {
				return nil, domain.ErrNotFound
			},
		},
	})
	_, err := h.Execute(context.Background(), Command{RefreshToken: "token", Now: now})
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got: %v", err)
	}
}
