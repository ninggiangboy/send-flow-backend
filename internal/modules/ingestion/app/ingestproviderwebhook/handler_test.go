package ingestproviderwebhook

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/ingestion/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/ingestion/ports"
)

// --- Stubs ---

type stubProviderRegistry struct {
	verifierFn   func(provider string) (ports.ProviderVerifier, bool)
	normalizerFn func(provider string) (ports.ProviderNormalizer, bool)
}

func (s *stubProviderRegistry) Verifier(provider string) (ports.ProviderVerifier, bool) {
	return s.verifierFn(provider)
}

func (s *stubProviderRegistry) Normalizer(provider string) (ports.ProviderNormalizer, bool) {
	return s.normalizerFn(provider)
}

type stubProviderVerifier struct {
	verifyFn func(ctx context.Context, input ports.VerifyInput) error
}

func (s *stubProviderVerifier) Verify(ctx context.Context, input ports.VerifyInput) error {
	return s.verifyFn(ctx, input)
}

type stubProviderNormalizer struct {
	normalizeFn func(ctx context.Context, input ports.NormalizeInput) (*ports.NormalizedProviderEventInput, error)
}

func (s *stubProviderNormalizer) Normalize(ctx context.Context, input ports.NormalizeInput) (*ports.NormalizedProviderEventInput, error) {
	return s.normalizeFn(ctx, input)
}

type stubRawEventWriteRepo struct {
	createFn func(ctx context.Context, event domain.ProviderWebhookEvent) error
}

func (s *stubRawEventWriteRepo) Create(ctx context.Context, event domain.ProviderWebhookEvent) error {
	return s.createFn(ctx, event)
}

type stubRawEventReadRepo struct {
	findByIDFn              func(ctx context.Context, id string) (*domain.ProviderWebhookEvent, error)
	findByProviderEventIDFn func(ctx context.Context, provider, providerEventID string) (*domain.ProviderWebhookEvent, error)
}

func (s *stubRawEventReadRepo) FindByID(ctx context.Context, id string) (*domain.ProviderWebhookEvent, error) {
	if s.findByIDFn != nil {
		return s.findByIDFn(ctx, id)
	}
	return nil, nil
}

func (s *stubRawEventReadRepo) FindByProviderEventID(ctx context.Context, provider, providerEventID string) (*domain.ProviderWebhookEvent, error) {
	return s.findByProviderEventIDFn(ctx, provider, providerEventID)
}

type stubNormalizedEventWriteRepo struct {
	createFn func(ctx context.Context, event domain.NormalizedProviderEvent) error
}

func (s *stubNormalizedEventWriteRepo) Create(ctx context.Context, event domain.NormalizedProviderEvent) error {
	return s.createFn(ctx, event)
}

type stubNormalizedEventReadRepo struct {
	findByIDFn              func(ctx context.Context, id string) (*domain.NormalizedProviderEvent, error)
	findByProviderEventIDFn func(ctx context.Context, provider, providerEventID, eventType string) (*domain.NormalizedProviderEvent, error)
}

func (s *stubNormalizedEventReadRepo) FindByID(ctx context.Context, id string) (*domain.NormalizedProviderEvent, error) {
	if s.findByIDFn != nil {
		return s.findByIDFn(ctx, id)
	}
	return nil, nil
}

func (s *stubNormalizedEventReadRepo) FindByProviderEventID(ctx context.Context, provider, providerEventID, eventType string) (*domain.NormalizedProviderEvent, error) {
	return s.findByProviderEventIDFn(ctx, provider, providerEventID, eventType)
}

type stubMessageResolver struct {
	findByProviderMessageIDFn func(ctx context.Context, provider, providerMessageID string) (*ports.MessageRef, error)
}

func (s *stubMessageResolver) FindByProviderMessageID(ctx context.Context, provider, providerMessageID string) (*ports.MessageRef, error) {
	return s.findByProviderMessageIDFn(ctx, provider, providerMessageID)
}

type stubOutboxWriter struct {
	saveFn func(ctx context.Context, event ports.OutboxEvent) error
}

func (s *stubOutboxWriter) Save(ctx context.Context, event ports.OutboxEvent) error {
	return s.saveFn(ctx, event)
}

type stubTxManager struct {
	withinTxFn func(ctx context.Context, fn func(ctx context.Context) error) error
}

func (s *stubTxManager) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return s.withinTxFn(ctx, fn)
}

// --- Helpers ---

func newTestOpts() Options {
	idCounter := 0

	return Options{
		RawEventsRead: &stubRawEventReadRepo{
			findByProviderEventIDFn: func(ctx context.Context, provider, providerEventID string) (*domain.ProviderWebhookEvent, error) {
				return &domain.ProviderWebhookEvent{ID: "existing_raw_evt"}, nil
			},
		},
		RawEventsWrite: &stubRawEventWriteRepo{
			createFn: func(ctx context.Context, event domain.ProviderWebhookEvent) error {
				return nil
			},
		},
		NormalizedEventsRead: &stubNormalizedEventReadRepo{
			findByProviderEventIDFn: func(ctx context.Context, provider, providerEventID, eventType string) (*domain.NormalizedProviderEvent, error) {
				return &domain.NormalizedProviderEvent{ID: "existing_norm_evt"}, nil
			},
		},
		NormalizedEventsWrite: &stubNormalizedEventWriteRepo{
			createFn: func(ctx context.Context, event domain.NormalizedProviderEvent) error {
				return nil
			},
		},
		ProviderRegistry: &stubProviderRegistry{
			verifierFn: func(provider string) (ports.ProviderVerifier, bool) {
				return &stubProviderVerifier{
					verifyFn: func(ctx context.Context, input ports.VerifyInput) error {
						return nil
					},
				}, true
			},
			normalizerFn: func(provider string) (ports.ProviderNormalizer, bool) {
				return &stubProviderNormalizer{
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
		MessageResolver: &stubMessageResolver{
			findByProviderMessageIDFn: func(ctx context.Context, provider, providerMessageID string) (*ports.MessageRef, error) {
				return &ports.MessageRef{WorkspaceID: "ws_1", MessageID: "msg_1"}, nil
			},
		},
		OutboxWriter: &stubOutboxWriter{
			saveFn: func(ctx context.Context, event ports.OutboxEvent) error {
				return nil
			},
		},
		TxManager: &stubTxManager{
			withinTxFn: func(ctx context.Context, fn func(ctx context.Context) error) error {
				return fn(ctx)
			},
		},
		IDGen: func() (string, error) {
			idCounter++
			return fmt.Sprintf("id_%d", idCounter), nil
		},
		Logger: slog.Default(),
	}
}

func defaultInput() Input {
	return Input{
		Provider: "ses",
		Headers: map[string][]string{
			"x-sendflow-fake-signature": {"sig1"},
		},
		RawBody:    []byte(`{"event":"test"}`),
		ReceivedAt: time.Now(),
	}
}

// --- Tests ---

func TestIngestProviderWebhook_Success(t *testing.T) {
	opts := newTestOpts()
	h := New(opts)

	result, err := h.Execute(context.Background(), defaultInput())
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

func TestIngestProviderWebhook_EmptyProvider(t *testing.T) {
	opts := newTestOpts()
	h := New(opts)

	input := defaultInput()
	input.Provider = ""

	_, err := h.Execute(context.Background(), input)
	if !errors.Is(err, domain.ErrProviderNotSupported) {
		t.Fatalf("expected ErrProviderNotSupported, got %v", err)
	}
}

func TestIngestProviderWebhook_UnsupportedProvider(t *testing.T) {
	opts := newTestOpts()
	reg := opts.ProviderRegistry.(*stubProviderRegistry)
	reg.verifierFn = func(provider string) (ports.ProviderVerifier, bool) {
		return nil, false
	}
	h := New(opts)

	input := defaultInput()
	input.Provider = "unknown"

	_, err := h.Execute(context.Background(), input)
	if !errors.Is(err, domain.ErrProviderNotSupported) {
		t.Fatalf("expected ErrProviderNotSupported, got %v", err)
	}
}

func TestIngestProviderWebhook_ProviderMissingNormalizer(t *testing.T) {
	opts := newTestOpts()
	reg := opts.ProviderRegistry.(*stubProviderRegistry)
	reg.normalizerFn = func(provider string) (ports.ProviderNormalizer, bool) {
		return nil, false
	}
	h := New(opts)

	_, err := h.Execute(context.Background(), defaultInput())
	if !errors.Is(err, domain.ErrProviderNotSupported) {
		t.Fatalf("expected ErrProviderNotSupported, got %v", err)
	}
}

func TestIngestProviderWebhook_PayloadExceedsMaxSize(t *testing.T) {
	opts := newTestOpts()
	h := New(opts)

	input := defaultInput()
	input.RawBody = make([]byte, 256*1024+1)

	_, err := h.Execute(context.Background(), input)
	if !errors.Is(err, domain.ErrPayloadInvalid) {
		t.Fatalf("expected ErrPayloadInvalid, got %v", err)
	}
}

func TestIngestProviderWebhook_InvalidJSON(t *testing.T) {
	opts := newTestOpts()
	h := New(opts)

	input := defaultInput()
	input.RawBody = []byte("not valid json")

	_, err := h.Execute(context.Background(), input)
	if !errors.Is(err, domain.ErrPayloadInvalid) {
		t.Fatalf("expected ErrPayloadInvalid, got %v", err)
	}
}

func TestIngestProviderWebhook_EmptyBody(t *testing.T) {
	opts := newTestOpts()
	h := New(opts)

	input := defaultInput()
	input.RawBody = []byte{}

	_, err := h.Execute(context.Background(), input)
	if !errors.Is(err, domain.ErrPayloadInvalid) {
		t.Fatalf("expected ErrPayloadInvalid, got %v", err)
	}
}

func TestIngestProviderWebhook_SignatureVerificationFailure(t *testing.T) {
	opts := newTestOpts()
	reg := opts.ProviderRegistry.(*stubProviderRegistry)
	reg.verifierFn = func(provider string) (ports.ProviderVerifier, bool) {
		return &stubProviderVerifier{
			verifyFn: func(ctx context.Context, input ports.VerifyInput) error {
				return domain.ErrInvalidSignature
			},
		}, true
	}
	h := New(opts)

	_, err := h.Execute(context.Background(), defaultInput())
	if !errors.Is(err, domain.ErrInvalidSignature) {
		t.Fatalf("expected ErrInvalidSignature, got %v", err)
	}
}

func TestIngestProviderWebhook_DuplicateEventReturnsExisting(t *testing.T) {
	opts := newTestOpts()

	rawWrite := opts.RawEventsWrite.(*stubRawEventWriteRepo)
	rawWrite.createFn = func(ctx context.Context, event domain.ProviderWebhookEvent) error {
		return domain.ErrDuplicateEventConflict
	}

	h := New(opts)

	result, err := h.Execute(context.Background(), defaultInput())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Accepted {
		t.Fatalf("expected Accepted=true, got false")
	}
	if result.RawEventID != "existing_raw_evt" {
		t.Fatalf("expected RawEventID 'existing_raw_evt', got %q", result.RawEventID)
	}
	if result.NormalizedEventID != "" {
		t.Fatalf("expected empty NormalizedEventID for idempotent raw event, got %q", result.NormalizedEventID)
	}
}

func TestIngestProviderWebhook_DuplicateEventErrorOnConflictCheck(t *testing.T) {
	opts := newTestOpts()

	rawWrite := opts.RawEventsWrite.(*stubRawEventWriteRepo)
	rawWrite.createFn = func(ctx context.Context, event domain.ProviderWebhookEvent) error {
		return domain.ErrDuplicateEventConflict
	}
	rawRead := opts.RawEventsRead.(*stubRawEventReadRepo)
	rawRead.findByProviderEventIDFn = func(ctx context.Context, provider, providerEventID string) (*domain.ProviderWebhookEvent, error) {
		return nil, errors.New("db lookup error")
	}

	h := New(opts)

	_, err := h.Execute(context.Background(), defaultInput())
	if !errors.Is(err, domain.ErrDuplicateEventConflict) {
		t.Fatalf("expected ErrDuplicateEventConflict, got %v", err)
	}
}

func TestIngestProviderWebhook_TransactionErrorPropagated(t *testing.T) {
	opts := newTestOpts()

	tx := opts.TxManager.(*stubTxManager)
	tx.withinTxFn = func(ctx context.Context, fn func(ctx context.Context) error) error {
		return errors.New("database connection lost")
	}

	h := New(opts)

	_, err := h.Execute(context.Background(), defaultInput())
	if !errors.Is(err, domain.ErrTemporarilyUnavailable) {
		t.Fatalf("expected ErrTemporarilyUnavailable, got %v", err)
	}
}

func TestIngestProviderWebhook_SignatureVerifierError(t *testing.T) {
	opts := newTestOpts()
	reg := opts.ProviderRegistry.(*stubProviderRegistry)
	reg.verifierFn = func(provider string) (ports.ProviderVerifier, bool) {
		return &stubProviderVerifier{
			verifyFn: func(ctx context.Context, input ports.VerifyInput) error {
				return errors.New("verifier internal error")
			},
		}, true
	}
	h := New(opts)

	_, err := h.Execute(context.Background(), defaultInput())
	if !errors.Is(err, domain.ErrTemporarilyUnavailable) {
		t.Fatalf("expected ErrTemporarilyUnavailable, got %v", err)
	}
}

func TestIngestProviderWebhook_NormalizationErrorSkipsNormalizedEvent(t *testing.T) {
	opts := newTestOpts()

	reg := opts.ProviderRegistry.(*stubProviderRegistry)
	reg.normalizerFn = func(provider string) (ports.ProviderNormalizer, bool) {
		return &stubProviderNormalizer{
			normalizeFn: func(ctx context.Context, input ports.NormalizeInput) (*ports.NormalizedProviderEventInput, error) {
				return nil, errors.New("normalization failed")
			},
		}, true
	}

	h := New(opts)

	result, err := h.Execute(context.Background(), defaultInput())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Accepted {
		t.Fatalf("expected Accepted=true, got false")
	}
	if result.RawEventID == "" {
		t.Fatalf("expected RawEventID to be set")
	}
	if result.NormalizedEventID != "" {
		t.Fatalf("expected empty NormalizedEventID when normalization fails, got %q", result.NormalizedEventID)
	}
}

func TestIngestProviderWebhook_MessageResolverNotFoundSkipsResolution(t *testing.T) {
	opts := newTestOpts()

	resolver := opts.MessageResolver.(*stubMessageResolver)
	resolver.findByProviderMessageIDFn = func(ctx context.Context, provider, providerMessageID string) (*ports.MessageRef, error) {
		return nil, nil
	}

	h := New(opts)

	result, err := h.Execute(context.Background(), defaultInput())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Accepted {
		t.Fatalf("expected Accepted=true, got false")
	}
}

func TestIngestProviderWebhook_RawEventWriteError(t *testing.T) {
	opts := newTestOpts()

	rawWrite := opts.RawEventsWrite.(*stubRawEventWriteRepo)
	rawWrite.createFn = func(ctx context.Context, event domain.ProviderWebhookEvent) error {
		return errors.New("database write failed")
	}

	h := New(opts)

	_, err := h.Execute(context.Background(), defaultInput())
	if !errors.Is(err, domain.ErrTemporarilyUnavailable) {
		t.Fatalf("expected ErrTemporarilyUnavailable, got %v", err)
	}
}
