package provider

import (
	"github.com/ninggiangboy/send-flow/backend/internal/modules/ingestion/ports"
)

type Registry struct {
	verifiers   map[string]ports.ProviderVerifier
	normalizers map[string]ports.ProviderNormalizer
}

func NewRegistry() *Registry {
	return &Registry{
		verifiers:   make(map[string]ports.ProviderVerifier),
		normalizers: make(map[string]ports.ProviderNormalizer),
	}
}

func (r *Registry) Register(provider string, verifier ports.ProviderVerifier, normalizer ports.ProviderNormalizer) {
	if verifier != nil {
		r.verifiers[provider] = verifier
	}
	if normalizer != nil {
		r.normalizers[provider] = normalizer
	}
}

func (r *Registry) Verifier(provider string) (ports.ProviderVerifier, bool) {
	v, ok := r.verifiers[provider]
	return v, ok
}

func (r *Registry) Normalizer(provider string) (ports.ProviderNormalizer, bool) {
	n, ok := r.normalizers[provider]
	return n, ok
}
