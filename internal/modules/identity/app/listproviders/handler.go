package listproviders

import (
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/usecase"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/ports"
)

type Handler struct {
	providers map[string]ports.OAuthProvider
}

func New(providers map[string]ports.OAuthProvider) *Handler {
	return &Handler{providers: providers}
}

func (h *Handler) Execute() []usecase.Provider {
	out := make([]usecase.Provider, 0, len(h.providers))
	for _, p := range h.providers {
		out = append(out, usecase.Provider{
			Provider:    p.Name(),
			Type:        p.Type(),
			DisplayName: p.DisplayName(),
			Enabled:     p.Enabled(),
		})
	}
	return out
}
