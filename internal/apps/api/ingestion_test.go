package api

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	ingestionapp "github.com/ninggiangboy/send-flow/backend/internal/modules/ingestion/app"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/ingestion/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/ingestion/ports"
	platformhealth "github.com/ninggiangboy/send-flow/backend/internal/platform/health"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/observability"
)

type memoryRawEventRepo struct {
	mu     sync.Mutex
	events []domain.ProviderWebhookEvent
	byID   map[string]domain.ProviderWebhookEvent
}

func newMemoryRawEventRepo() *memoryRawEventRepo {
	return &memoryRawEventRepo{byID: map[string]domain.ProviderWebhookEvent{}}
}

func (r *memoryRawEventRepo) FindByID(_ context.Context, id string) (*domain.ProviderWebhookEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.byID[id]
	if !ok {
		return nil, nil
	}
	return &e, nil
}

func (r *memoryRawEventRepo) Create(_ context.Context, event domain.ProviderWebhookEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event)
	r.byID[event.ID] = event
	return nil
}

func (r *memoryRawEventRepo) FindByProviderEventID(_ context.Context, provider, providerEventID string) (*domain.ProviderWebhookEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, e := range r.events {
		if e.Provider == provider && e.ProviderEventID == providerEventID {
			return &e, nil
		}
	}
	return nil, nil
}

type memoryNormalizedEventRepo struct {
	mu     sync.Mutex
	events []domain.NormalizedProviderEvent
	byID   map[string]domain.NormalizedProviderEvent
}

func newMemoryNormalizedEventRepo() *memoryNormalizedEventRepo {
	return &memoryNormalizedEventRepo{byID: map[string]domain.NormalizedProviderEvent{}}
}

func (r *memoryNormalizedEventRepo) FindByID(_ context.Context, id string) (*domain.NormalizedProviderEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.byID[id]
	if !ok {
		return nil, nil
	}
	return &e, nil
}

func (r *memoryNormalizedEventRepo) Create(_ context.Context, event domain.NormalizedProviderEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event)
	r.byID[event.ID] = event
	return nil
}

func (r *memoryNormalizedEventRepo) FindByProviderEventID(_ context.Context, provider, providerEventID, eventType string) (*domain.NormalizedProviderEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, e := range r.events {
		if e.Provider == provider && e.ProviderEventID == providerEventID && e.EventType == eventType {
			return &e, nil
		}
	}
	return nil, nil
}

type memoryProviderRegistry struct {
	mu          sync.Mutex
	verifiers   map[string]ports.ProviderVerifier
	normalizers map[string]ports.ProviderNormalizer
}

func newMemoryProviderRegistry() *memoryProviderRegistry {
	r := &memoryProviderRegistry{
		verifiers:   map[string]ports.ProviderVerifier{},
		normalizers: map[string]ports.ProviderNormalizer{},
	}
	r.Register("fake", acceptAllVerifier{}, passthroughNormalizer{})
	return r
}

func (r *memoryProviderRegistry) Verifier(name string) (ports.ProviderVerifier, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	v, ok := r.verifiers[name]
	return v, ok
}

func (r *memoryProviderRegistry) Normalizer(name string) (ports.ProviderNormalizer, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	n, ok := r.normalizers[name]
	return n, ok
}

func (r *memoryProviderRegistry) Register(name string, verifier ports.ProviderVerifier, normalizer ports.ProviderNormalizer) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.verifiers[name] = verifier
	r.normalizers[name] = normalizer
}

type acceptAllVerifier struct{}

func (acceptAllVerifier) Verify(_ context.Context, _ ports.VerifyInput) error { return nil }

type passthroughNormalizer struct{}

func (passthroughNormalizer) Normalize(_ context.Context, input ports.NormalizeInput) (*ports.NormalizedProviderEventInput, error) {
	return &ports.NormalizedProviderEventInput{
		ProviderEventID: "prov-ev-1",
		EventType:       "bounce",
		OccurredAt:      time.Now(),
		PayloadJSON:     input.RawBody,
	}, nil
}

type memoryIngestionOutboxWriter struct{}

func (memoryIngestionOutboxWriter) Save(_ context.Context, _ ports.OutboxEvent) error { return nil }

type memoryIngestionTxManager struct{}

func (memoryIngestionTxManager) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

type memoryIngestionMessageResolver struct{}

func (memoryIngestionMessageResolver) FindByProviderMessageID(_ context.Context, _, _ string) (*ports.MessageRef, error) {
	return nil, nil
}

func newTestIngestionService() *ingestionapp.Service {
	return ingestionapp.NewService(ingestionapp.Options{
		RawEventsRead:         newMemoryRawEventRepo(),
		RawEventsWrite:        newMemoryRawEventRepo(),
		NormalizedEventsRead:  newMemoryNormalizedEventRepo(),
		NormalizedEventsWrite: newMemoryNormalizedEventRepo(),
		ProviderRegistry:      newMemoryProviderRegistry(),
		MessageResolver:       memoryIngestionMessageResolver{},
		OutboxWriter:          memoryIngestionOutboxWriter{},
		TxManager:             memoryIngestionTxManager{},
		IDGen:                 func() (string, error) { return "test-event-id", nil },
		Logger:                slog.Default(),
	})
}

func setupIngestionRouter(t *testing.T) http.Handler {
	t.Helper()
	svc := newTestIngestionService()
	healthSvc := platformhealth.NewService(platformhealth.Options{
		AppName:       "sendflow",
		PostgresCheck: func(context.Context) error { return nil },
		RedisCheck:    func(context.Context) error { return nil },
	})
	metrics, _ := observability.NewHTTPMetrics(nil)
	return newRouter(&RouterDeps{
		HealthSvc:       healthSvc,
		AuthSvc:         nil,
		IngestionSvc:    svc,
		SecureCookies:   false,
		FrontendBaseURL: "http://localhost:3000",
		HTTPMetrics:     metrics,
		Log:             slog.Default(),
	})
}

func TestIngestionWebhook_Success(t *testing.T) {
	router := setupIngestionRouter(t)

	rec := httptest.NewRecorder()
	body := `{"event": "bounce", "email": "bounce@example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/providers/fake", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestIngestionWebhook_ProviderNotSupported(t *testing.T) {
	router := setupIngestionRouter(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/providers/unknown", strings.NewReader(`{"event":"bounce"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestIngestionWebhook_EmptyBody(t *testing.T) {
	router := setupIngestionRouter(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/providers/fake", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}
