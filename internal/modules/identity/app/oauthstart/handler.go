package oauthstart

import (
	"context"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/usecase"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/ports"
)

type Handler struct {
	deps usecase.Deps
	log  *slog.Logger
}

type Command struct {
	Provider      string
	RedirectURI   string
	Intent        string
	CodeChallenge string
	CodeVerifier  string
	Now           time.Time
}

func New(deps usecase.Deps) *Handler {
	return &Handler{deps: deps, log: deps.Logger.With("usecase", "oauth_start")}
}

func (h *Handler) Execute(ctx context.Context, cmd Command) (*usecase.OAuthStartResult, error) {
	p, ok := h.deps.Providers[cmd.Provider]
	if !ok {
		h.log.Warn("unknown OAuth provider requested", "provider", cmd.Provider)
		return nil, domain.ErrNotFound
	}
	if !p.Enabled() {
		h.log.Warn("disabled OAuth provider requested", "provider", cmd.Provider)
		return nil, domain.ErrProviderDisabled
	}
	state, err := h.deps.IDGen.New()
	if err != nil {
		h.log.Error("failed to generate OAuth state", "provider", cmd.Provider, "error", err)
		return nil, err
	}
	if err := h.deps.OAuthState.Save(ctx, state, ports.OAuthState{Provider: cmd.Provider, RedirectURI: cmd.RedirectURI, Intent: cmd.Intent, CodeVerifier: cmd.CodeVerifier, CreatedAt: cmd.Now}, h.deps.OAuthStateTTL); err != nil {
		h.log.Error("failed to save OAuth state", "provider", cmd.Provider, "error", err)
		return nil, err
	}
	h.log.Info("OAuth flow initiated", "provider", cmd.Provider)
	return &usecase.OAuthStartResult{Provider: cmd.Provider, AuthorizationURL: p.BuildAuthURL(state, cmd.RedirectURI, cmd.CodeChallenge), State: state, CodeChallengeMethod: "S256"}, nil
}
