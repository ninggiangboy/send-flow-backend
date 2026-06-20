package app

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/ingestion/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/ingestion/ports"
)

// minimal stubs for facade-level tests
type svcStubTxManager struct {
	withinTxFn func(ctx context.Context, fn func(ctx context.Context) error) error
}

func (s *svcStubTxManager) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return s.withinTxFn(ctx, fn)
}

type svcStubProviderVerifier struct {
	verifyFn func(ctx context.Context, input ports.VerifyInput) error
}

func (s *svcStubProviderVerifier) Verify(ctx context.Context, input ports.VerifyInput) error {
	return s.verifyFn(ctx, input)
}

type svcStubProviderNormalizer struct {
	normalizeFn func(ctx context.Context, input ports.NormalizeInput) (*ports.NormalizedProviderEventInput, error)
}

func (s *svcStubProviderNormalizer) Normalize(ctx context.Context, input ports.NormalizeInput) (*ports.NormalizedProviderEventInput, error) {
	return s.normalizeFn(ctx, input)
}

type svcStubProviderRegistry struct {
	verifierFn   func(provider string) (ports.ProviderVerifier, bool)
	normalizerFn func(provider string) (ports.ProviderNormalizer, bool)
}

func (s *svcStubProviderRegistry) Verifier(provider string) (ports.ProviderVerifier, bool) {
	return s.verifierFn(provider)
}

func (s *svcStubProviderRegistry) Normalizer(provider string) (ports.ProviderNormalizer, bool) {
	return s.normalizerFn(provider)
}

type svcStubRawEventWriteRepo struct {
	createFn                func(ctx context.Context, event domain.ProviderWebhookEvent) error
	findByIDFn              func(ctx context.Context, id string) (*domain.ProviderWebhookEvent, error)
	findByProviderEventIDFn func(ctx context.Context, provider, providerEventID string) (*domain.ProviderWebhookEvent, error)
}

func (s *svcStubRawEventWriteRepo) Create(ctx context.Context, event domain.ProviderWebhookEvent) error {
	return s.createFn(ctx, event)
}

func (s *svcStubRawEventWriteRepo) FindByID(ctx context.Context, id string) (*domain.ProviderWebhookEvent, error) {
	if s.findByIDFn != nil {
		return s.findByIDFn(ctx, id)
	}
	return nil, nil
}

func (s *svcStubRawEventWriteRepo) FindByProviderEventID(ctx context.Context, provider, providerEventID string) (*domain.ProviderWebhookEvent, error) {
	if s.findByProviderEventIDFn != nil {
		return s.findByProviderEventIDFn(ctx, provider, providerEventID)
	}
	return nil, nil
}

type svcStubRawEventReadRepo struct {
	findByProviderEventIDFn func(ctx context.Context, provider, providerEventID string) (*domain.ProviderWebhookEvent, error)
}

func (s *svcStubRawEventReadRepo) FindByID(ctx context.Context, id string) (*domain.ProviderWebhookEvent, error) {
	return nil, nil
}

func (s *svcStubRawEventReadRepo) FindByProviderEventID(ctx context.Context, provider, providerEventID string) (*domain.ProviderWebhookEvent, error) {
	return s.findByProviderEventIDFn(ctx, provider, providerEventID)
}

type svcStubNormalizedEventWriteRepo struct {
	createFn                func(ctx context.Context, event domain.NormalizedProviderEvent) error
	findByIDFn              func(ctx context.Context, id string) (*domain.NormalizedProviderEvent, error)
	findByProviderEventIDFn func(ctx context.Context, provider, providerEventID, eventType string) (*domain.NormalizedProviderEvent, error)
}

func (s *svcStubNormalizedEventWriteRepo) Create(ctx context.Context, event domain.NormalizedProviderEvent) error {
	return s.createFn(ctx, event)
}

func (s *svcStubNormalizedEventWriteRepo) FindByID(ctx context.Context, id string) (*domain.NormalizedProviderEvent, error) {
	if s.findByIDFn != nil {
		return s.findByIDFn(ctx, id)
	}
	return nil, nil
}

func (s *svcStubNormalizedEventWriteRepo) FindByProviderEventID(ctx context.Context, provider, providerEventID, eventType string) (*domain.NormalizedProviderEvent, error) {
	if s.findByProviderEventIDFn != nil {
		return s.findByProviderEventIDFn(ctx, provider, providerEventID, eventType)
	}
	return nil, nil
}

type svcStubNormalizedEventReadRepo struct {
	findByProviderEventIDFn func(ctx context.Context, provider, providerEventID, eventType string) (*domain.NormalizedProviderEvent, error)
}

func (s *svcStubNormalizedEventReadRepo) FindByID(ctx context.Context, id string) (*domain.NormalizedProviderEvent, error) {
	return nil, nil
}

func (s *svcStubNormalizedEventReadRepo) FindByProviderEventID(ctx context.Context, provider, providerEventID, eventType string) (*domain.NormalizedProviderEvent, error) {
	return s.findByProviderEventIDFn(ctx, provider, providerEventID, eventType)
}

type svcStubMessageResolver struct {
	findByProviderMessageIDFn func(ctx context.Context, provider, providerMessageID string) (*ports.MessageRef, error)
}

func (s *svcStubMessageResolver) FindByProviderMessageID(ctx context.Context, provider, providerMessageID string) (*ports.MessageRef, error) {
	return s.findByProviderMessageIDFn(ctx, provider, providerMessageID)
}

type svcStubOutboxWriter struct {
	saveFn func(ctx context.Context, event ports.OutboxEvent) error
}

func (s *svcStubOutboxWriter) Save(ctx context.Context, event ports.OutboxEvent) error {
	return s.saveFn(ctx, event)
}

func TestService_IngestProviderWebhook_Success(t *testing.T) {
	idCounter := 0
	svc := NewService(Options{
		RawEventsRead: &svcStubRawEventReadRepo{
			findByProviderEventIDFn: func(ctx context.Context, provider, providerEventID string) (*domain.ProviderWebhookEvent, error) {
				return nil, nil
			},
		},
		RawEventsWrite: &svcStubRawEventWriteRepo{
			createFn: func(ctx context.Context, event domain.ProviderWebhookEvent) error {
				return nil
			},
		},
		NormalizedEventsRead: &svcStubNormalizedEventReadRepo{
			findByProviderEventIDFn: func(ctx context.Context, provider, providerEventID, eventType string) (*domain.NormalizedProviderEvent, error) {
				return nil, nil
			},
		},
		NormalizedEventsWrite: &svcStubNormalizedEventWriteRepo{
			createFn: func(ctx context.Context, event domain.NormalizedProviderEvent) error {
				return nil
			},
		},
		ProviderRegistry: &svcStubProviderRegistry{
			verifierFn: func(provider string) (ports.ProviderVerifier, bool) {
				return &svcStubProviderVerifier{
					verifyFn: func(ctx context.Context, input ports.VerifyInput) error {
						return nil
					},
				}, true
			},
			normalizerFn: func(provider string) (ports.ProviderNormalizer, bool) {
				return &svcStubProviderNormalizer{
					normalizeFn: func(ctx context.Context, input ports.NormalizeInput) (*ports.NormalizedProviderEventInput, error) {
						return &ports.NormalizedProviderEventInput{
							ProviderEventID:   "prov_evt_1",
							ProviderMessageID: "prov_msg_1",
							EventType:         domain.EventTypeDelivered,
							OccurredAt:        time.Now(),
							PayloadJSON:       input.RawBody,
						}, nil
					},
				}, true
			},
		},
		MessageResolver: &svcStubMessageResolver{
			findByProviderMessageIDFn: func(ctx context.Context, provider, providerMessageID string) (*ports.MessageRef, error) {
				return &ports.MessageRef{WorkspaceID: "ws_1", MessageID: "msg_1"}, nil
			},
		},
		OutboxWriter: &svcStubOutboxWriter{
			saveFn: func(ctx context.Context, event ports.OutboxEvent) error {
				return nil
			},
		},
		TxManager: &svcStubTxManager{
			withinTxFn: func(ctx context.Context, fn func(ctx context.Context) error) error {
				return fn(ctx)
			},
		},
		IDGen: func() (string, error) {
			idCounter++
			return "id_1", nil
		},
		Logger: slog.Default(),
	})

	result, err := svc.IngestProviderWebhook(context.Background(), IngestProviderWebhookInput{
		Provider: "ses",
		Headers: map[string][]string{
			"x-sendflow-fake-signature": {"sig1"},
		},
		RawBody:    []byte(`{"event":"test"}`),
		ReceivedAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Accepted {
		t.Fatalf("expected Accepted=true, got false")
	}
	if result.RawEventID == "" {
		t.Fatalf("expected RawEventID to be set")
	}
	if result.NormalizedEventID == "" {
		t.Fatalf("expected NormalizedEventID to be set")
	}
}

func TestService_IngestProviderWebhook_ErrorPropagated(t *testing.T) {
	svc := NewService(Options{
		RawEventsRead:         &svcStubRawEventReadRepo{},
		RawEventsWrite:        &svcStubRawEventWriteRepo{},
		NormalizedEventsRead:  &svcStubNormalizedEventReadRepo{},
		NormalizedEventsWrite: &svcStubNormalizedEventWriteRepo{},
		ProviderRegistry: &svcStubProviderRegistry{
			verifierFn: func(provider string) (ports.ProviderVerifier, bool) {
				return nil, false
			},
			normalizerFn: func(provider string) (ports.ProviderNormalizer, bool) {
				return nil, false
			},
		},
		MessageResolver: &svcStubMessageResolver{},
		OutboxWriter:    &svcStubOutboxWriter{},
		TxManager:       &svcStubTxManager{},
		IDGen:           func() (string, error) { return "", nil },
		Logger:          slog.Default(),
	})

	_, err := svc.IngestProviderWebhook(context.Background(), IngestProviderWebhookInput{
		Provider: "unknown",
	})
	if !errors.Is(err, domain.ErrProviderNotSupported) {
		t.Fatalf("expected ErrProviderNotSupported, got %v", err)
	}
}
