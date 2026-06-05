package authenticate

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/usecase"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/ports"
)

type tokenStub struct {
	claims *ports.AccessClaims
	err    error
}

func (s *tokenStub) Issue(string, string, time.Time) (ports.TokenPair, string, string, error) {
	return ports.TokenPair{}, "", "", nil
}
func (s *tokenStub) ParseAccess(string) (*ports.AccessClaims, error)  { return s.claims, s.err }
func (s *tokenStub) ParseRefresh(string) (*ports.AccessClaims, error) { return s.claims, s.err }

func TestExecuteUnauthorizedWhenTokenInvalid(t *testing.T) {
	h := New(usecase.Deps{Tokens: &tokenStub{err: errors.New("bad token")}})
	_, _, err := h.Execute(context.Background(), "token")
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("expected unauthorized, got %v", err)
	}
}

func TestExecuteSuccess(t *testing.T) {
	h := New(usecase.Deps{
		Tokens: &tokenStub{claims: &ports.AccessClaims{Subject: "u1", SessionID: "s1", JWTID: "j1", ExpiresAt: time.Now().UTC().Add(1 * time.Hour).Unix()}},
	})
	sess, user, err := h.Execute(context.Background(), "token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sess.ID != "s1" || sess.UserID != "u1" || sess.AccessJTI != "j1" || user.ID != "u1" {
		t.Fatalf("unexpected result: %v %v", sess, user)
	}
}

func TestExecuteUnauthorizedWhenClaimsIncomplete(t *testing.T) {
	h := New(usecase.Deps{
		Tokens: &tokenStub{claims: &ports.AccessClaims{Subject: "u1", JWTID: "j1"}},
	})
	_, _, err := h.Execute(context.Background(), "token")
	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("expected unauthorized for incomplete claims, got %v", err)
	}
}
