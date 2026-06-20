package auth

import (
	"testing"

	"context"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/ports"
)

type authProviderStub struct{ enabled bool }

func (s *authProviderStub) Name() string        { return "google" }
func (s *authProviderStub) Type() string        { return "oauth2" }
func (s *authProviderStub) DisplayName() string { return "Google" }
func (s *authProviderStub) Enabled() bool       { return s.enabled }
func (s *authProviderStub) BuildAuthURL(string, string, string) string {
	return "https://example.com/auth"
}
func (s *authProviderStub) Exchange(context.Context, string, string, string) (*domain.OAuthIdentity, error) {
	return nil, nil
}

func TestListProvidersHandlerExecuteReturnsConfiguredProviders(t *testing.T) {
	h := NewListProvidersHandler(map[string]ports.OAuthProvider{
		"google": &authProviderStub{enabled: true},
	})

	out := h.Execute()
	if len(out) != 1 || out[0].Provider != "google" || !out[0].Enabled {
		t.Fatalf("unexpected providers: %+v", out)
	}
}
