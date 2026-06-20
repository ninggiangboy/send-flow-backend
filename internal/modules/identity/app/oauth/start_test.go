package oauth

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/ports"
)

func oauthTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type oauthProviderTestStub struct{ enabled bool }

func (s *oauthProviderTestStub) Name() string        { return "google" }
func (s *oauthProviderTestStub) Type() string        { return "oauth2" }
func (s *oauthProviderTestStub) DisplayName() string { return "Google" }
func (s *oauthProviderTestStub) Enabled() bool       { return s.enabled }
func (s *oauthProviderTestStub) BuildAuthURL(state, _, _ string) string {
	return "https://example.com/auth?state=" + state
}
func (s *oauthProviderTestStub) Exchange(context.Context, string, string, string) (*domain.OAuthIdentity, error) {
	return nil, nil
}

type oauthIDGenStub struct{ id string }

func (s oauthIDGenStub) New() (string, error) { return s.id, nil }

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

func TestStartHandlerExecuteProviderDisabled(t *testing.T) {
	h := NewStartHandler(StartOptions{
		Providers: map[string]ports.OAuthProvider{"google": &oauthProviderTestStub{enabled: false}},
		Logger:    oauthTestLogger(),
	})
	_, err := h.Execute(context.Background(), StartCommand{Provider: "google", Now: time.Now()})
	if !errors.Is(err, domain.ErrProviderDisabled) {
		t.Fatalf("expected provider disabled, got %v", err)
	}
}

func TestStartHandlerExecuteSuccess(t *testing.T) {
	store := &oauthStateStoreStub{}
	h := NewStartHandler(StartOptions{
		IdGen:         oauthIDGenStub{id: "state-1"},
		Providers:     map[string]ports.OAuthProvider{"google": &oauthProviderTestStub{enabled: true}},
		OauthState:    store,
		OauthStateTTL: 5 * time.Minute,
		Logger:        oauthTestLogger(),
	})
	result, err := h.Execute(context.Background(), StartCommand{Provider: "google", RedirectURI: "http://localhost/cb", Now: time.Now()})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.calls != 1 || !strings.Contains(result.AuthorizationURL, "state=") {
		t.Fatalf("unexpected result: %+v", result)
	}
}
