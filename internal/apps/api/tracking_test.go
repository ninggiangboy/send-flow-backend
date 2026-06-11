package api

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	trackingapp "github.com/ninggiangboy/send-flow/backend/internal/modules/tracking/app"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/tracking/app/unsubscribetoken"
	trackingdomain "github.com/ninggiangboy/send-flow/backend/internal/modules/tracking/domain"
	platformhealth "github.com/ninggiangboy/send-flow/backend/internal/platform/health"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/observability"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/outbox"
)

type memoryTrackingLinkRepo struct {
	mu    sync.Mutex
	links map[string]trackingdomain.TrackingLink
}

func newMemoryTrackingLinkRepo() *memoryTrackingLinkRepo {
	return &memoryTrackingLinkRepo{links: map[string]trackingdomain.TrackingLink{}}
}

func (r *memoryTrackingLinkRepo) FindByID(_ context.Context, id string) (*trackingdomain.TrackingLink, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	l, ok := r.links[id]
	if !ok {
		return nil, trackingdomain.ErrTrackingLinkNotFound
	}
	return &l, nil
}

func (r *memoryTrackingLinkRepo) ListByMessage(_ context.Context, _, _ string) ([]trackingdomain.TrackingLink, error) {
	return nil, nil
}

func (r *memoryTrackingLinkRepo) Create(_ context.Context, link trackingdomain.TrackingLink) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.links[link.ID] = link
	return nil
}

type memoryTrackingEventRepo struct {
	mu     sync.Mutex
	events []trackingdomain.TrackingEvent
}

func newMemoryTrackingEventRepo() *memoryTrackingEventRepo {
	return &memoryTrackingEventRepo{}
}

func (r *memoryTrackingEventRepo) FindBySourceEvent(_ context.Context, source, sourceEventID, eventType string) (*trackingdomain.TrackingEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, e := range r.events {
		if e.Source == source && e.SourceEventID == sourceEventID && e.EventType == eventType {
			return &e, nil
		}
	}
	return nil, nil
}

func (r *memoryTrackingEventRepo) ListByMessage(_ context.Context, _, _ string, _ int, _ string) ([]trackingdomain.TrackingEvent, string, error) {
	return nil, "", nil
}

func (r *memoryTrackingEventRepo) Create(_ context.Context, event trackingdomain.TrackingEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event)
	return nil
}

type memoryDeliveryMessageResolver struct{}

func (memoryDeliveryMessageResolver) FindByID(_ context.Context, _, _ string) (string, string, string, string, string, string, error) {
	return "", "", "", "", "", "", nil
}

func (memoryDeliveryMessageResolver) FindByProviderMessageID(_ context.Context, _, _ string) (string, string, string, error) {
	return "", "", "", nil
}

type memoryRecipientSuppressor struct{}

func (memoryRecipientSuppressor) SuppressFromSignal(_ context.Context, _ trackingapp.SuppressFromSignalInput) (*trackingapp.SuppressFromSignalResult, error) {
	return &trackingapp.SuppressFromSignalResult{EntryID: "sup-1", Created: true}, nil
}

type memoryOutboxWriter struct {
	mu sync.Mutex
}

func (w *memoryOutboxWriter) Save(_ context.Context, _ outbox.Event) error {
	return nil
}

type memoryTxManager struct{}

func (memoryTxManager) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

func newTestTrackingService() *trackingapp.Service {
	return trackingapp.NewService(trackingapp.Options{
		LinkReadRepo:        newMemoryTrackingLinkRepo(),
		LinkWriteRepo:       newMemoryTrackingLinkRepo(),
		EventReadRepo:       newMemoryTrackingEventRepo(),
		EventWriteRepo:      newMemoryTrackingEventRepo(),
		MessageResolver:     memoryDeliveryMessageResolver{},
		RecipientSuppressor: memoryRecipientSuppressor{},
		OutboxWriter:        &memoryOutboxWriter{},
		TxManager:           memoryTxManager{},
		IDGen:               func() (string, error) { return "test-event-id", nil },
		Logger:              slog.Default(),
		TokenSigner:         unsubscribetoken.NewSigner("test-secret"),
	})
}

func setupTrackingRouter(t *testing.T) (http.Handler, *trackingapp.Service) {
	t.Helper()
	svc := newTestTrackingService()
	healthSvc := platformhealth.NewService(platformhealth.Options{
		AppName:       "sendflow",
		PostgresCheck: func(context.Context) error { return nil },
		RedisCheck:    func(context.Context) error { return nil },
	})
	metrics, _ := observability.NewHTTPMetrics(nil)
	return newRouter(&RouterDeps{
		HealthSvc:       healthSvc,
		TrackingSvc:     svc,
		SecureCookies:   false,
		FrontendBaseURL: "http://localhost:3000",
		HTTPMetrics:     metrics,
		Log:             slog.Default(),
	}), svc
}

func TestTrackingOpenPixel_Success(t *testing.T) {
	router, _ := setupTrackingRouter(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/o/track-123", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "image/gif" {
		t.Fatalf("expected Content-Type image/gif, got %q", ct)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-store, no-cache, must-revalidate" {
		t.Fatalf("expected no-cache Cache-Control, got %q", cc)
	}
	if len(rec.Body.Bytes()) == 0 {
		t.Fatal("expected non-empty pixel body")
	}
}

func TestTrackingOpenPixel_Headers(t *testing.T) {
	router, _ := setupTrackingRouter(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/o/track-123", nil)
	router.ServeHTTP(rec, req)

	if h := rec.Header().Get("Pragma"); h != "no-cache" {
		t.Fatalf("expected Pragma: no-cache, got %q", h)
	}
	if h := rec.Header().Get("Expires"); h != "0" {
		t.Fatalf("expected Expires: 0, got %q", h)
	}
}

func TestTrackingClickRedirect_MissingLink(t *testing.T) {
	router, _ := setupTrackingRouter(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/t/missing-link", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for missing link, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestTrackingClickRedirect_EmptyTrackingID(t *testing.T) {
	router, _ := setupTrackingRouter(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/t/", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestTrackingUnsubscribe_Success(t *testing.T) {
	router, _ := setupTrackingRouter(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/u/test-unsub-token", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "unsubscribed") {
		t.Fatalf("expected unsubscribed confirmation in body, got %q", body)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("expected text/html content type, got %q", ct)
	}
}

func TestTrackingUnsubscribe_EmptyToken(t *testing.T) {
	router, _ := setupTrackingRouter(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/u/", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", rec.Code, rec.Body.String())
	}
}
