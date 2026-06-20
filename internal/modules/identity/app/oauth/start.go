package oauth

import (
	"context"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/shared"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/ports"
)

type StartOptions struct {
	Providers     map[string]ports.OAuthProvider
	IdGen         ports.IDGenerator
	OauthState    ports.OAuthStateStore
	OauthStateTTL time.Duration
	Logger        *slog.Logger
}

type StartHandler struct {
	providers     map[string]ports.OAuthProvider
	idGen         ports.IDGenerator
	oauthState    ports.OAuthStateStore
	oauthStateTTL time.Duration
	log           *slog.Logger
}

type StartCommand struct {
	Provider      string
	RedirectURI   string
	Intent        string
	CodeChallenge string
	CodeVerifier  string
	Now           time.Time
}

func NewStartHandler(opts StartOptions) *StartHandler {
	return &StartHandler{
		providers:     opts.Providers,
		idGen:         opts.IdGen,
		oauthState:    opts.OauthState,
		oauthStateTTL: opts.OauthStateTTL,
		log:           opts.Logger.With("usecase", "oauth_start"),
	}
}

func (h *StartHandler) Execute(ctx context.Context, cmd StartCommand) (*shared.OAuthStartResult, error) {
	p, ok := h.providers[cmd.Provider]
	if !ok {
		h.log.Warn("unknown OAuth provider requested", "provider", cmd.Provider)
		return nil, domain.ErrNotFound
	}
	if !p.Enabled() {
		h.log.Warn("disabled OAuth provider requested", "provider", cmd.Provider)
		return nil, domain.ErrProviderDisabled
	}
	state, err := h.idGen.New()
	if err != nil {
		h.log.Error("failed to generate OAuth state", "provider", cmd.Provider, "error", err)
		return nil, err
	}
	if err := h.oauthState.Save(ctx, state, ports.OAuthState{Provider: cmd.Provider, RedirectURI: cmd.RedirectURI, Intent: cmd.Intent, CodeVerifier: cmd.CodeVerifier, CreatedAt: cmd.Now}, h.oauthStateTTL); err != nil {
		h.log.Error("failed to save OAuth state", "provider", cmd.Provider, "error", err)
		return nil, err
	}
	h.log.Info("OAuth flow initiated", "provider", cmd.Provider)
	return &shared.OAuthStartResult{Provider: cmd.Provider, AuthorizationURL: p.BuildAuthURL(state, cmd.RedirectURI, cmd.CodeChallenge), State: state, CodeChallengeMethod: "S256"}, nil
}
