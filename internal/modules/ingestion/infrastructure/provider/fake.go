package provider

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"strings"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/ingestion/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/ingestion/ports"
)

type FakeVerifier struct {
	Secret string
}

func (v *FakeVerifier) Verify(_ context.Context, input ports.VerifyInput) error {
	sig := ""
	for k, vals := range input.Headers {
		if strings.EqualFold(k, "X-Sendflow-Fake-Signature") {
			if len(vals) > 0 {
				sig = vals[0]
			}
			break
		}
	}
	if v.Secret != "" && subtle.ConstantTimeCompare([]byte(sig), []byte(v.Secret)) != 1 {
		return domain.ErrInvalidSignature
	}
	return nil
}

type fakePayload struct {
	EventID           string `json:"event_id"`
	ProviderMessageID string `json:"provider_message_id"`
	MessageID         string `json:"message_id"`
	EventType         string `json:"event_type"`
	OccurredAt        string `json:"occurred_at"`
}

type FakeNormalizer struct{}

func (n *FakeNormalizer) Normalize(_ context.Context, input ports.NormalizeInput) (*ports.NormalizedProviderEventInput, error) {
	var p fakePayload
	if err := json.Unmarshal(input.RawBody, &p); err != nil {
		return nil, domain.ErrPayloadInvalid
	}
	if p.EventType == "" {
		return nil, domain.ErrPayloadInvalid
	}
	if p.ProviderMessageID == "" && p.MessageID == "" {
		return nil, domain.ErrPayloadInvalid
	}
	if !domain.ValidEventType(p.EventType) {
		return nil, domain.ErrPayloadInvalid
	}

	occurredAt := time.Now().UTC()
	if p.OccurredAt != "" {
		t, err := time.Parse(time.RFC3339, p.OccurredAt)
		if err == nil {
			occurredAt = t.UTC()
		}
	}

	providerMessageID := p.ProviderMessageID
	if providerMessageID == "" {
		providerMessageID = p.MessageID
	}

	return &ports.NormalizedProviderEventInput{
		ProviderEventID:   p.EventID,
		ProviderMessageID: providerMessageID,
		EventType:         p.EventType,
		OccurredAt:        occurredAt,
		PayloadJSON:       input.RawBody,
	}, nil
}

type FakeProviderRegistry struct {
	providerVerifier   ports.ProviderVerifier
	providerNormalizer ports.ProviderNormalizer
}

func NewFakeProviderRegistry(verifier ports.ProviderVerifier, normalizer ports.ProviderNormalizer) *FakeProviderRegistry {
	return &FakeProviderRegistry{
		providerVerifier:   verifier,
		providerNormalizer: normalizer,
	}
}

func (r *FakeProviderRegistry) Verifier(provider string) (ports.ProviderVerifier, bool) {
	if provider == domain.ProviderFake {
		return r.providerVerifier, true
	}
	return nil, false
}

func (r *FakeProviderRegistry) Normalizer(provider string) (ports.ProviderNormalizer, bool) {
	if provider == domain.ProviderFake {
		return r.providerNormalizer, true
	}
	return nil, false
}
