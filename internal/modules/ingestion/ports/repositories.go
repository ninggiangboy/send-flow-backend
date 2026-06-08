package ports

import (
	"context"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/ingestion/domain"
)

type RawEventReadRepository interface {
	FindByID(ctx context.Context, id string) (*domain.ProviderWebhookEvent, error)
	FindByProviderEventID(ctx context.Context, provider, providerEventID string) (*domain.ProviderWebhookEvent, error)
}

type RawEventWriteRepository interface {
	Create(ctx context.Context, event domain.ProviderWebhookEvent) error
}

type NormalizedEventReadRepository interface {
	FindByID(ctx context.Context, id string) (*domain.NormalizedProviderEvent, error)
	FindByProviderEventID(ctx context.Context, provider, providerEventID, eventType string) (*domain.NormalizedProviderEvent, error)
}

type NormalizedEventWriteRepository interface {
	Create(ctx context.Context, event domain.NormalizedProviderEvent) error
}

type MessageRef struct {
	WorkspaceID       string
	MessageID         string
	Provider          string
	ProviderMessageID string
}

type DeliveryMessageResolver interface {
	FindByProviderMessageID(ctx context.Context, provider, providerMessageID string) (*MessageRef, error)
}

type OutboxEvent struct {
	ID            string
	AggregateType string
	AggregateID   string
	EventType     string
	Payload       []byte
	Headers       map[string]string
	WorkspaceID   string
	OccurredAt    time.Time
}

type OutboxWriter interface {
	Save(ctx context.Context, event OutboxEvent) error
}

type TransactionManager interface {
	RunInTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}

type VerifyInput struct {
	Provider string
	Headers  map[string][]string
	RawBody  []byte
}

type ProviderVerifier interface {
	Verify(ctx context.Context, input VerifyInput) error
}

type NormalizeInput struct {
	Provider string
	RawBody  []byte
}

type NormalizedProviderEventInput struct {
	ProviderEventID   string
	ProviderMessageID string
	EventType         string
	OccurredAt        time.Time
	PayloadJSON       []byte
}

type ProviderNormalizer interface {
	Normalize(ctx context.Context, input NormalizeInput) (*NormalizedProviderEventInput, error)
}

type ProviderRegistry interface {
	Verifier(provider string) (ProviderVerifier, bool)
	Normalizer(provider string) (ProviderNormalizer, bool)
}
