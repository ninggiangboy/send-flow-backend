package auth

import (
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/shared"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/ports"
)

type ListProvidersHandler struct {
	providers map[string]ports.OAuthProvider
}

func NewListProvidersHandler(providers map[string]ports.OAuthProvider) *ListProvidersHandler {
	return &ListProvidersHandler{providers: providers}
}

func (h *ListProvidersHandler) Execute() []shared.Provider {
	out := make([]shared.Provider, 0, len(h.providers))
	for _, p := range h.providers {
		out = append(out, shared.Provider{
			Provider:    p.Name(),
			Type:        p.Type(),
			DisplayName: p.DisplayName(),
			Enabled:     p.Enabled(),
		})
	}
	return out
}
