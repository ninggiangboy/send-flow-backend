package oauthstart

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/usecase"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/ports"
)

var testLogger = slog.Default()

type oauthProviderStub struct{ enabled bool }

func (s *oauthProviderStub) Name() string        { return "google" }
func (s *oauthProviderStub) Type() string        { return "oauth2" }
func (s *oauthProviderStub) DisplayName() string { return "Google" }
func (s *oauthProviderStub) Enabled() bool       { return s.enabled }
func (s *oauthProviderStub) BuildAuthURL(state, _, _ string) string {
	return "https://example.com/auth?state=" + state
}
func (s *oauthProviderStub) Exchange(context.Context, string, string, string) (*domain.OAuthIdentity, error) {
	return nil, nil
}

type idGenStub struct {
	id string
}

func (s idGenStub) New() (string, error) { return s.id, nil }

type oauthStateStoreStub struct {
	calls int
	err   error
}

func (s *oauthStateStoreStub) Save(context.Context, string, ports.OAuthState, time.Duration) error {
	s.calls++
	return s.err
}
func (s *oauthStateStoreStub) GetAndDelete(context.Context, string) (*ports.OAuthState, error) {
	return nil, nil
}

func TestExecuteProviderDisabled(t *testing.T) {
	h := New(usecase.Deps{Providers: map[string]ports.OAuthProvider{"google": &oauthProviderStub{enabled: false}}, Logger: testLogger})
	_, err := h.Execute(context.Background(), Command{Provider: "google", Now: time.Now().UTC()})
	if !errors.Is(err, domain.ErrProviderDisabled) {
		t.Fatalf("expected provider disabled, got %v", err)
	}
}

func TestExecuteSuccess(t *testing.T) {
	store := &oauthStateStoreStub{}
	h := New(usecase.Deps{
		IDGen:         idGenStub{id: "state-1"},
		Providers:     map[string]ports.OAuthProvider{"google": &oauthProviderStub{enabled: true}},
		OAuthState:    store,
		OAuthStateTTL: 5 * time.Minute,
		Logger:        testLogger,
	})
	res, err := h.Execute(context.Background(), Command{Provider: "google", RedirectURI: "http://localhost/cb", Now: time.Now().UTC()})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.calls != 1 || !strings.Contains(res.AuthorizationURL, "state=") {
		t.Fatalf("unexpected result: %+v", res)
	}
}
