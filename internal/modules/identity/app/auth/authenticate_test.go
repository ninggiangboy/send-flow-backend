package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/ports"
)

func TestAuthenticateHandlerExecuteReturnsUnauthorizedOnParseError(t *testing.T) {
	h := NewAuthenticateHandler(&authTokenManagerStub{
		parseAccess: func(string, time.Time) (*ports.AccessClaims, error) { return nil, errors.New("bad token") },
	}, &authSessionReadStub{}, testLogger())

	_, _, err := h.Execute(context.Background(), "token", time.Now())
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
}

func TestAuthenticateHandlerExecuteReturnsSessionAndUser(t *testing.T) {
	now := time.Now().UTC()
	h := NewAuthenticateHandler(&authTokenManagerStub{
		parseAccess: func(string, time.Time) (*ports.AccessClaims, error) {
			return &ports.AccessClaims{Subject: "user_1", SessionID: "sess_1"}, nil
		},
	}, &authSessionReadStub{
		findByID: func(context.Context, string) (*domain.Session, error) {
			return &domain.Session{ID: "sess_1", UserID: "user_1", ExpiresAt: now.Add(time.Hour)}, nil
		},
	}, testLogger())

	sess, user, err := h.Execute(context.Background(), "token", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sess.ID != "sess_1" || user.ID != "user_1" {
		t.Fatalf("unexpected result: sess=%+v user=%+v", sess, user)
	}
}
