package listproviders

import "github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/usecase"

type Handler struct{ deps usecase.Deps }

func New(deps usecase.Deps) *Handler { return &Handler{deps: deps} }

func (h *Handler) Execute() []usecase.Provider {
	out := make([]usecase.Provider, 0, len(h.deps.Providers))
	for _, p := range h.deps.Providers {
		out = append(out, usecase.Provider{Provider: p.Name(), Type: p.Type(), DisplayName: p.DisplayName(), Enabled: p.Enabled()})
	}
	return out
}
