package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/ports"
)

func TestRefreshHandlerExecuteReturnsUnauthorizedForMissingRefreshJTI(t *testing.T) {
	h := NewRefreshHandler(RefreshOptions{
		Tokens: &authTokenManagerStub{
			parseRefresh: func(string, time.Time) (*ports.AccessClaims, error) {
				return &ports.AccessClaims{JWTID: "rjti", SessionID: "sess_1"}, nil
			},
		},
		RefreshStore: &authRefreshStoreStub{},
		Logger:       testLogger(),
	})

	_, err := h.Execute(context.Background(), RefreshCommand{RefreshToken: "rt", Now: time.Now()})
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
}

func TestRefreshHandlerExecuteRotatesSession(t *testing.T) {
	now := time.Now().UTC()
	var rotated bool
	var replaced bool
	h := NewRefreshHandler(RefreshOptions{
		Tokens: &authTokenManagerStub{
			parseRefresh: func(string, time.Time) (*ports.AccessClaims, error) {
				return &ports.AccessClaims{JWTID: "old-rjti", SessionID: "sess_1"}, nil
			},
			issue: func(userID, sessionID string, now time.Time) (ports.TokenPair, string, string, error) {
				return ports.TokenPair{AccessToken: "at", RefreshToken: "rt", AccessExpiresAt: now.Add(time.Hour), RefreshExpiresAt: now.Add(24 * time.Hour)}, "new-ajti", "new-rjti", nil
			},
		},
		RefreshStore: &authRefreshStoreStub{
			find: func(context.Context, string) (string, error) { return "sess_1", nil },
			replace: func(context.Context, string, string, string, time.Duration) error {
				replaced = true
				return nil
			},
		},
		SessionsWrite: &authSessionWriteStub{
			findByID: func(context.Context, string) (*domain.Session, error) {
				return &domain.Session{ID: "sess_1", UserID: "u1", RefreshJTI: "old-rjti", ExpiresAt: now.Add(time.Hour)}, nil
			},
			rotateTokens: func(context.Context, string, string, string, time.Time, time.Time) error {
				rotated = true
				return nil
			},
		},
		UsersWrite: &authUserWriteStub{
			findByID: func(context.Context, string) (*domain.User, error) {
				return &domain.User{ID: "u1", Email: "u@example.com"}, nil
			},
		},
		Logger: testLogger(),
	})

	result, err := h.Execute(context.Background(), RefreshCommand{RefreshToken: "rt", Now: now})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Session.ID != "sess_1" || !rotated || !replaced {
		t.Fatalf("unexpected refresh result: %+v rotated=%v replaced=%v", result, rotated, replaced)
	}
}
