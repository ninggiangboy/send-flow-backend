package listproviders

import (
	"context"
	"testing"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/usecase"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/ports"
)

type providerStub struct {
	name        string
	providerTyp string
	displayName string
	enabled     bool
}

func (p *providerStub) Name() string        { return p.name }
func (p *providerStub) Type() string        { return p.providerTyp }
func (p *providerStub) DisplayName() string { return p.displayName }
func (p *providerStub) Enabled() bool       { return p.enabled }
func (p *providerStub) BuildAuthURL(state, redirectURI, codeChallenge string) string {
	return ""
}
func (p *providerStub) Exchange(_ context.Context, code, redirectURI, codeVerifier string) (*domain.OAuthIdentity, error) {
	return nil, nil
}

func TestListProvidersEmpty(t *testing.T) {
	h := New(usecase.Deps{
		Providers: map[string]ports.OAuthProvider{},
	})
	result := h.Execute()
	if len(result) != 0 {
		t.Fatalf("expected empty result, got %d items", len(result))
	}
}

func TestListProviders(t *testing.T) {
	h := New(usecase.Deps{
		Providers: map[string]ports.OAuthProvider{
			"google": &providerStub{name: "google", providerTyp: "oauth2", displayName: "Google", enabled: true},
			"github": &providerStub{name: "github", providerTyp: "oauth2", displayName: "GitHub", enabled: false},
		},
	})
	result := h.Execute()
	if len(result) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(result))
	}
	found := map[string]bool{}
	for _, p := range result {
		found[p.Provider] = true
		if p.Provider == "google" && (!p.Enabled || p.DisplayName != "Google") {
			t.Fatalf("unexpected google provider: %+v", p)
		}
		if p.Provider == "github" && (p.Enabled || p.DisplayName != "GitHub") {
			t.Fatalf("unexpected github provider: %+v", p)
		}
	}
	if !found["google"] || !found["github"] {
		t.Fatal("missing expected providers")
	}
}
