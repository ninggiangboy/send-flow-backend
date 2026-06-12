package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	analyticsapp "github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/app"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/domain"
	platformhealth "github.com/ninggiangboy/send-flow/backend/internal/platform/health"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/observability"
)

type stubCampaignQueryRepo struct {
	GetCampaignFunnelFunc     func(ctx context.Context, workspaceID, campaignID string, from, to time.Time) (*domain.CampaignFunnel, error)
	GetCampaignTimeSeriesFunc func(ctx context.Context, workspaceID, campaignID string, from, to time.Time, interval, eventType string) (*domain.CampaignTimeSeriesResult, error)
	GetCampaignBreakdownFunc  func(ctx context.Context, workspaceID, campaignID string, from, to time.Time, groupBy string) (*domain.CampaignBreakdownResult, error)
	GetCampaignEventsFunc     func(ctx context.Context, filter domain.CampaignQueryFilter) (*domain.CampaignEventsResult, error)
}

func (s *stubCampaignQueryRepo) GetCampaignFunnel(ctx context.Context, workspaceID, campaignID string, from, to time.Time) (*domain.CampaignFunnel, error) {
	return s.GetCampaignFunnelFunc(ctx, workspaceID, campaignID, from, to)
}
func (s *stubCampaignQueryRepo) GetCampaignTimeSeries(ctx context.Context, workspaceID, campaignID string, from, to time.Time, interval, eventType string) (*domain.CampaignTimeSeriesResult, error) {
	return s.GetCampaignTimeSeriesFunc(ctx, workspaceID, campaignID, from, to, interval, eventType)
}
func (s *stubCampaignQueryRepo) GetCampaignBreakdown(ctx context.Context, workspaceID, campaignID string, from, to time.Time, groupBy string) (*domain.CampaignBreakdownResult, error) {
	return s.GetCampaignBreakdownFunc(ctx, workspaceID, campaignID, from, to, groupBy)
}
func (s *stubCampaignQueryRepo) GetCampaignEvents(ctx context.Context, filter domain.CampaignQueryFilter) (*domain.CampaignEventsResult, error) {
	return s.GetCampaignEventsFunc(ctx, filter)
}

type nopAccessChecker struct{}

func (nopAccessChecker) RequirePermission(_ context.Context, _, _, _ string) error {
	return nil
}

func setupAnalyticsRouter(t *testing.T, queryRepo *stubCampaignQueryRepo) http.Handler {
	t.Helper()

	svc := analyticsapp.NewService(analyticsapp.Options{
		CampaignQueryRepo: queryRepo,
		Logger:            slog.Default(),
	})

	healthSvc := platformhealth.NewService(platformhealth.Options{
		AppName:       "sendflow",
		PostgresCheck: func(context.Context) error { return nil },
		RedisCheck:    func(context.Context) error { return nil },
	})
	metrics, _ := observability.NewHTTPMetrics(nil)
	return newRouter(&RouterDeps{
		HealthSvc:       healthSvc,
		AnalyticsSvc:    svc,
		SecureCookies:   false,
		FrontendBaseURL: "http://localhost:3000",
		HTTPMetrics:     metrics,
		Log:             slog.Default(),
	})
}

type analyticsEnvelope struct {
	Data json.RawMessage `json:"data"`
}

func TestAnalyticsGetCampaignFunnel_Success(t *testing.T) {
	router := setupAnalyticsRouter(t, &stubCampaignQueryRepo{
		GetCampaignFunnelFunc: func(_ context.Context, _, _ string, _, _ time.Time) (*domain.CampaignFunnel, error) {
			return &domain.CampaignFunnel{
				Status:         "ready",
				WorkspaceID:    "ws-1",
				CampaignID:     "camp-1",
				DeliveredCount: 100,
				AcceptedCount:  120,
				BouncedCount:   5,
			}, nil
		},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/ws-1/analytics/campaigns/camp-1/funnel", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var env analyticsEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	var result campaignFunnelResponseDoc
	if err := json.Unmarshal(env.Data, &result); err != nil {
		t.Fatalf("failed to unmarshal data: %v", err)
	}
	if result.Status != "ready" {
		t.Fatalf("expected status ready, got %s", result.Status)
	}
	if result.DeliveredCount != 100 {
		t.Fatalf("expected 100 delivered, got %d", result.DeliveredCount)
	}
}

func TestAnalyticsGetCampaignFunnel_InvalidTimeRange(t *testing.T) {
	router := setupAnalyticsRouter(t, &stubCampaignQueryRepo{})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/ws-1/analytics/campaigns/camp-1/funnel?from=2025-01-02T00:00:00Z&to=2025-01-01T00:00:00Z", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAnalyticsGetCampaignTimeSeries_Success(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	router := setupAnalyticsRouter(t, &stubCampaignQueryRepo{
		GetCampaignTimeSeriesFunc: func(_ context.Context, _, _ string, _, _ time.Time, _, _ string) (*domain.CampaignTimeSeriesResult, error) {
			return &domain.CampaignTimeSeriesResult{
				Status:      "ready",
				WorkspaceID: "ws-1",
				CampaignID:  "camp-1",
				Buckets: []domain.CampaignTimeSeriesBucket{
					{BucketStart: fixedTime, EventType: "delivered", Count: 50},
				},
			}, nil
		},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/ws-1/analytics/campaigns/camp-1/timeseries?interval=day", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var env analyticsEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	var result campaignTimeSeriesResponseDoc
	if err := json.Unmarshal(env.Data, &result); err != nil {
		t.Fatalf("failed to unmarshal data: %v", err)
	}
	if result.Status != "ready" {
		t.Fatalf("expected status ready, got %s", result.Status)
	}
	if len(result.Buckets) != 1 {
		t.Fatalf("expected 1 bucket, got %d", len(result.Buckets))
	}
}

func TestAnalyticsGetCampaignTimeSeries_InvalidInterval(t *testing.T) {
	router := setupAnalyticsRouter(t, &stubCampaignQueryRepo{})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/ws-1/analytics/campaigns/camp-1/timeseries?interval=month", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAnalyticsGetCampaignBreakdown_Success(t *testing.T) {
	router := setupAnalyticsRouter(t, &stubCampaignQueryRepo{
		GetCampaignBreakdownFunc: func(_ context.Context, _, _ string, _, _ time.Time, _ string) (*domain.CampaignBreakdownResult, error) {
			return &domain.CampaignBreakdownResult{
				Status:      "ready",
				WorkspaceID: "ws-1",
				CampaignID:  "camp-1",
				GroupBy:     "event_type",
				Rows: []domain.CampaignBreakdownRow{
					{GroupKey: "sendgrid", EventType: "delivered", Count: 100},
				},
			}, nil
		},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/ws-1/analytics/campaigns/camp-1/breakdown?group_by=event_type", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var env analyticsEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	var result campaignBreakdownResponseDoc
	if err := json.Unmarshal(env.Data, &result); err != nil {
		t.Fatalf("failed to unmarshal data: %v", err)
	}
	if result.Status != "ready" {
		t.Fatalf("expected status ready, got %s", result.Status)
	}
	if len(result.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(result.Rows))
	}
	if result.GroupBy != "event_type" {
		t.Fatalf("expected group_by event_type, got %s", result.GroupBy)
	}
}

func TestAnalyticsGetCampaignBreakdown_InvalidGroup(t *testing.T) {
	router := setupAnalyticsRouter(t, &stubCampaignQueryRepo{})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/ws-1/analytics/campaigns/camp-1/breakdown?group_by=unknown", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAnalyticsGetCampaignEvents_Success(t *testing.T) {
	router := setupAnalyticsRouter(t, &stubCampaignQueryRepo{
		GetCampaignEventsFunc: func(_ context.Context, _ domain.CampaignQueryFilter) (*domain.CampaignEventsResult, error) {
			return &domain.CampaignEventsResult{
				Status:      "ready",
				WorkspaceID: "ws-1",
				CampaignID:  "camp-1",
				Events: []domain.CampaignEventRow{
					{
						SourceEventID:   "evt-1",
						SourceEventType: "test.event",
						EventType:       "delivered",
						MessageID:       "msg-1",
						OccurredAt:      "2025-01-15T10:00:00Z",
						ReceivedAt:      "2025-01-15T10:00:01Z",
					},
				},
			}, nil
		},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/ws-1/analytics/campaigns/camp-1/events?limit=10", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var env analyticsEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	var result campaignEventsResponseDoc
	if err := json.Unmarshal(env.Data, &result); err != nil {
		t.Fatalf("failed to unmarshal data: %v", err)
	}
	if result.Status != "ready" {
		t.Fatalf("expected status ready, got %s", result.Status)
	}
	if len(result.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(result.Events))
	}
	if result.Events[0].SourceEventID != "evt-1" {
		t.Fatalf("expected evt-1, got %s", result.Events[0].SourceEventID)
	}
}

func TestAnalyticsGetCampaignEvents_FilterByEventType(t *testing.T) {
	var capturedFilter domain.CampaignQueryFilter

	router := setupAnalyticsRouter(t, &stubCampaignQueryRepo{
		GetCampaignEventsFunc: func(_ context.Context, filter domain.CampaignQueryFilter) (*domain.CampaignEventsResult, error) {
			capturedFilter = filter
			return &domain.CampaignEventsResult{
				Status:      "ready",
				WorkspaceID: "ws-1",
				CampaignID:  "camp-1",
			}, nil
		},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/ws-1/analytics/campaigns/camp-1/events?event_type=delivered&limit=10", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	if capturedFilter.EventType != "delivered" {
		t.Fatalf("expected event_type delivered, got %s", capturedFilter.EventType)
	}
	if capturedFilter.Limit != 10 {
		t.Fatalf("expected limit 10, got %d", capturedFilter.Limit)
	}
}

// ---------------------------------------------------------------------------
// Stubs for forensics and operations
// ---------------------------------------------------------------------------

type stubAPIForensicRepo struct {
	SearchEventsFunc                func(ctx context.Context, filter domain.ForensicQueryFilter) (*domain.ForensicEventsResult, error)
	GetMessageTimelineFunc          func(ctx context.Context, workspaceID, messageID string) (*domain.MessageTimelineResult, error)
	GetProviderEventTraceFunc       func(ctx context.Context, workspaceID, providerEventID string) (*domain.ProviderEventTrace, error)
	GetCampaignIncidentTimelineFunc func(ctx context.Context, workspaceID, campaignID string, from, to time.Time) (*domain.CampaignIncidentTimelineResult, error)
}

func (s *stubAPIForensicRepo) SearchEvents(ctx context.Context, filter domain.ForensicQueryFilter) (*domain.ForensicEventsResult, error) {
	return s.SearchEventsFunc(ctx, filter)
}
func (s *stubAPIForensicRepo) GetMessageTimeline(ctx context.Context, workspaceID, messageID string) (*domain.MessageTimelineResult, error) {
	return s.GetMessageTimelineFunc(ctx, workspaceID, messageID)
}
func (s *stubAPIForensicRepo) GetProviderEventTrace(ctx context.Context, workspaceID, providerEventID string) (*domain.ProviderEventTrace, error) {
	return s.GetProviderEventTraceFunc(ctx, workspaceID, providerEventID)
}
func (s *stubAPIForensicRepo) GetCampaignIncidentTimeline(ctx context.Context, workspaceID, campaignID string, from, to time.Time) (*domain.CampaignIncidentTimelineResult, error) {
	return s.GetCampaignIncidentTimelineFunc(ctx, workspaceID, campaignID, from, to)
}

func setupAnalyticsRouterWithForensic(t *testing.T, forensicRepo *stubAPIForensicRepo) http.Handler {
	t.Helper()
	svc := analyticsapp.NewService(analyticsapp.Options{
		ForensicQueryRepo: forensicRepo,
		Logger:            slog.Default(),
	})
	healthSvc := platformhealth.NewService(platformhealth.Options{
		AppName:       "sendflow",
		PostgresCheck: func(context.Context) error { return nil },
		RedisCheck:    func(context.Context) error { return nil },
	})
	metrics, _ := observability.NewHTTPMetrics(nil)
	return newRouter(&RouterDeps{
		HealthSvc:       healthSvc,
		AnalyticsSvc:    svc,
		SecureCookies:   false,
		FrontendBaseURL: "http://localhost:3000",
		HTTPMetrics:     metrics,
		Log:             slog.Default(),
	})
}

// ---------------------------------------------------------------------------
// Forensics: SearchEvents
// ---------------------------------------------------------------------------

func TestAnalyticsSearchEvents_Success(t *testing.T) {
	router := setupAnalyticsRouterWithForensic(t, &stubAPIForensicRepo{
		SearchEventsFunc: func(_ context.Context, _ domain.ForensicQueryFilter) (*domain.ForensicEventsResult, error) {
			return &domain.ForensicEventsResult{
				Status:      "ready",
				WorkspaceID: "ws-1",
				Events: []domain.ForensicEventRow{
					{
						SourceEventID:   "evt-1",
						SourceEventType: "test.event",
						WorkspaceID:     "ws-1",
						CampaignID:      "camp-1",
						MessageID:       "msg-1",
						EventType:       "delivered",
						OccurredAt:      "2025-01-15T10:00:00Z",
						ReceivedAt:      "2025-01-15T10:00:01Z",
					},
				},
			}, nil
		},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/ws-1/analytics/events?from=2025-01-01T00:00:00Z&to=2025-01-31T00:00:00Z&limit=10", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var env analyticsEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	var result forensicEventsResponseDoc
	if err := json.Unmarshal(env.Data, &result); err != nil {
		t.Fatalf("failed to unmarshal data: %v", err)
	}
	if result.Status != "ready" {
		t.Fatalf("expected status ready, got %s", result.Status)
	}
	if len(result.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(result.Events))
	}
	if result.Events[0].SourceEventID != "evt-1" {
		t.Fatalf("expected evt-1, got %s", result.Events[0].SourceEventID)
	}
}

func TestAnalyticsSearchEvents_UnboundedReturns400(t *testing.T) {
	router := setupAnalyticsRouterWithForensic(t, &stubAPIForensicRepo{})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/ws-1/analytics/events", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Forensics: GetMessageTimeline
// ---------------------------------------------------------------------------

func TestAnalyticsGetMessageTimeline_Success(t *testing.T) {
	router := setupAnalyticsRouterWithForensic(t, &stubAPIForensicRepo{
		GetMessageTimelineFunc: func(_ context.Context, _, _ string) (*domain.MessageTimelineResult, error) {
			return &domain.MessageTimelineResult{
				Status:      "ready",
				WorkspaceID: "ws-1",
				MessageID:   "msg-1",
				Events: []domain.MessageTimelineRow{
					{SourceEventID: "evt-1", EventType: "accepted", OccurredAt: "2025-01-15T09:00:00Z", ReceivedAt: "2025-01-15T09:00:01Z"},
					{SourceEventID: "evt-2", EventType: "delivered", OccurredAt: "2025-01-15T09:01:00Z", ReceivedAt: "2025-01-15T09:01:01Z"},
				},
			}, nil
		},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/ws-1/analytics/messages/msg-1/timeline", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var env analyticsEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	var result messageTimelineResponseDoc
	if err := json.Unmarshal(env.Data, &result); err != nil {
		t.Fatalf("failed to unmarshal data: %v", err)
	}
	if result.Status != "ready" {
		t.Fatalf("expected status ready, got %s", result.Status)
	}
	if len(result.Events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(result.Events))
	}
}

// ---------------------------------------------------------------------------
// Forensics: GetProviderEventTrace
// ---------------------------------------------------------------------------

func TestAnalyticsGetProviderEventTrace_Success(t *testing.T) {
	router := setupAnalyticsRouterWithForensic(t, &stubAPIForensicRepo{
		GetProviderEventTraceFunc: func(_ context.Context, _, _ string) (*domain.ProviderEventTrace, error) {
			return &domain.ProviderEventTrace{
				Status:            "ready",
				ProviderEventID:   "pe-1",
				WorkspaceID:       "ws-1",
				CampaignID:        "camp-1",
				MessageID:         "msg-1",
				Provider:          "sendgrid",
				ProviderMessageID: "sg-msg-1",
			}, nil
		},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/ws-1/analytics/provider-events/pe-1", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var env analyticsEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	var result providerEventTraceResponseDoc
	if err := json.Unmarshal(env.Data, &result); err != nil {
		t.Fatalf("failed to unmarshal data: %v", err)
	}
	if result.Status != "ready" {
		t.Fatalf("expected status ready, got %s", result.Status)
	}
	if result.ProviderEventID != "pe-1" {
		t.Fatalf("expected pe-1, got %s", result.ProviderEventID)
	}
}

func TestAnalyticsGetProviderEventTrace_NotFound(t *testing.T) {
	router := setupAnalyticsRouterWithForensic(t, &stubAPIForensicRepo{
		GetProviderEventTraceFunc: func(_ context.Context, _, _ string) (*domain.ProviderEventTrace, error) {
			return nil, nil
		},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/ws-1/analytics/provider-events/unknown", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", rec.Code, rec.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Forensics: GetCampaignIncidentTimeline
// ---------------------------------------------------------------------------

func TestAnalyticsGetCampaignIncidentTimeline_Success(t *testing.T) {
	router := setupAnalyticsRouterWithForensic(t, &stubAPIForensicRepo{
		GetCampaignIncidentTimelineFunc: func(_ context.Context, _, _ string, _, _ time.Time) (*domain.CampaignIncidentTimelineResult, error) {
			return &domain.CampaignIncidentTimelineResult{
				Status:      "ready",
				WorkspaceID: "ws-1",
				CampaignID:  "camp-1",
				Events: []domain.CampaignIncidentTimelineRow{
					{SourceEventID: "evt-1", SourceEventType: "test.event", EventType: "bounced", MessageID: "msg-1", Provider: "sendgrid", OccurredAt: "2025-01-15T10:00:00Z"},
				},
			}, nil
		},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/ws-1/analytics/campaigns/camp-1/incident-timeline?from=2025-01-01T00:00:00Z&to=2025-01-31T00:00:00Z", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var env analyticsEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	var result campaignIncidentTimelineResponseDoc
	if err := json.Unmarshal(env.Data, &result); err != nil {
		t.Fatalf("failed to unmarshal data: %v", err)
	}
	if result.Status != "ready" {
		t.Fatalf("expected status ready, got %s", result.Status)
	}
	if len(result.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(result.Events))
	}
}

// ---------------------------------------------------------------------------
// Stubs for operations
// ---------------------------------------------------------------------------

type stubAPIOperationsRepo struct {
	GetOutboxLagFunc                 func(ctx context.Context, workspaceID string, from, to time.Time, source string) (*domain.OutboxLagResult, error)
	GetConsumerFailuresFunc          func(ctx context.Context, workspaceID string, from, to time.Time, source string) (*domain.ConsumerFailureResult, error)
	GetDLQVolumeFunc                 func(ctx context.Context, workspaceID string, from, to time.Time, source string) (*domain.DLQResult, error)
	GetWebhookDeliveryTimeSeriesFunc func(ctx context.Context, workspaceID string, from, to time.Time, status, interval string) (*domain.WebhookDeliveryTimeSeriesResult, error)
	GetWebhookReliabilityFunc        func(ctx context.Context, workspaceID string, from, to time.Time, target string) (*domain.WebhookReliabilityResult, error)
}

func (s *stubAPIOperationsRepo) GetOutboxLag(ctx context.Context, workspaceID string, from, to time.Time, source string) (*domain.OutboxLagResult, error) {
	return s.GetOutboxLagFunc(ctx, workspaceID, from, to, source)
}
func (s *stubAPIOperationsRepo) GetConsumerFailures(ctx context.Context, workspaceID string, from, to time.Time, source string) (*domain.ConsumerFailureResult, error) {
	return s.GetConsumerFailuresFunc(ctx, workspaceID, from, to, source)
}
func (s *stubAPIOperationsRepo) GetDLQVolume(ctx context.Context, workspaceID string, from, to time.Time, source string) (*domain.DLQResult, error) {
	return s.GetDLQVolumeFunc(ctx, workspaceID, from, to, source)
}
func (s *stubAPIOperationsRepo) GetWebhookDeliveryTimeSeries(ctx context.Context, workspaceID string, from, to time.Time, status, interval string) (*domain.WebhookDeliveryTimeSeriesResult, error) {
	return s.GetWebhookDeliveryTimeSeriesFunc(ctx, workspaceID, from, to, status, interval)
}
func (s *stubAPIOperationsRepo) GetWebhookReliability(ctx context.Context, workspaceID string, from, to time.Time, target string) (*domain.WebhookReliabilityResult, error) {
	return s.GetWebhookReliabilityFunc(ctx, workspaceID, from, to, target)
}

func setupAnalyticsRouterWithOperations(t *testing.T, opsRepo *stubAPIOperationsRepo) http.Handler {
	t.Helper()
	svc := analyticsapp.NewService(analyticsapp.Options{
		OperationsQueryRepo: opsRepo,
		Logger:              slog.Default(),
	})
	healthSvc := platformhealth.NewService(platformhealth.Options{
		AppName:       "sendflow",
		PostgresCheck: func(context.Context) error { return nil },
		RedisCheck:    func(context.Context) error { return nil },
	})
	metrics, _ := observability.NewHTTPMetrics(nil)
	return newRouter(&RouterDeps{
		HealthSvc:       healthSvc,
		AnalyticsSvc:    svc,
		SecureCookies:   false,
		FrontendBaseURL: "http://localhost:3000",
		HTTPMetrics:     metrics,
		Log:             slog.Default(),
	})
}

// ---------------------------------------------------------------------------
// Operations: GetOutboxLag
// ---------------------------------------------------------------------------

func TestAnalyticsGetOutboxLag_Success(t *testing.T) {
	router := setupAnalyticsRouterWithOperations(t, &stubAPIOperationsRepo{
		GetOutboxLagFunc: func(_ context.Context, _ string, _, _ time.Time, _ string) (*domain.OutboxLagResult, error) {
			return &domain.OutboxLagResult{
				Status:      "ready",
				WorkspaceID: "ws-1",
				Rows: []domain.OutboxLagRow{
					{Source: "delivery", EventType: "send_email", LagSeconds: 2.5, Count: 100, BucketStart: "2025-01-15T10:00:00Z"},
				},
			}, nil
		},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/ws-1/analytics/operations/outbox-lag", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var env analyticsEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	var result outboxLagResponseDoc
	if err := json.Unmarshal(env.Data, &result); err != nil {
		t.Fatalf("failed to unmarshal data: %v", err)
	}
	if result.Status != "ready" {
		t.Fatalf("expected status ready, got %s", result.Status)
	}
	if len(result.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(result.Rows))
	}
}

// ---------------------------------------------------------------------------
// Operations: GetConsumerFailures
// ---------------------------------------------------------------------------

func TestAnalyticsGetConsumerFailures_Success(t *testing.T) {
	router := setupAnalyticsRouterWithOperations(t, &stubAPIOperationsRepo{
		GetConsumerFailuresFunc: func(_ context.Context, _ string, _, _ time.Time, _ string) (*domain.ConsumerFailureResult, error) {
			return &domain.ConsumerFailureResult{
				Status:      "ready",
				WorkspaceID: "ws-1",
				Rows: []domain.ConsumerFailureRow{
					{Source: "delivery", Consumer: "analytics_events", ErrorType: "timeout", Count: 5, BucketStart: "2025-01-15T10:00:00Z"},
				},
			}, nil
		},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/ws-1/analytics/operations/consumer-failures", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var env analyticsEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	var result consumerFailureResponseDoc
	if err := json.Unmarshal(env.Data, &result); err != nil {
		t.Fatalf("failed to unmarshal data: %v", err)
	}
	if result.Status != "ready" {
		t.Fatalf("expected status ready, got %s", result.Status)
	}
	if len(result.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(result.Rows))
	}
}

// ---------------------------------------------------------------------------
// Operations: GetDLQVolume
// ---------------------------------------------------------------------------

func TestAnalyticsGetDLQVolume_Success(t *testing.T) {
	router := setupAnalyticsRouterWithOperations(t, &stubAPIOperationsRepo{
		GetDLQVolumeFunc: func(_ context.Context, _ string, _, _ time.Time, _ string) (*domain.DLQResult, error) {
			return &domain.DLQResult{
				Status:      "ready",
				WorkspaceID: "ws-1",
				Rows: []domain.DLQRow{
					{Source: "delivery", EventType: "send_email", Count: 10, BucketStart: "2025-01-15T10:00:00Z"},
				},
			}, nil
		},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/ws-1/analytics/operations/dlq", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var env analyticsEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	var result dlqResponseDoc
	if err := json.Unmarshal(env.Data, &result); err != nil {
		t.Fatalf("failed to unmarshal data: %v", err)
	}
	if result.Status != "ready" {
		t.Fatalf("expected status ready, got %s", result.Status)
	}
	if len(result.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(result.Rows))
	}
}

// ---------------------------------------------------------------------------
// Operations: GetWebhookDeliveryTimeSeries
// ---------------------------------------------------------------------------

func TestAnalyticsGetWebhookDeliveryTimeSeries_Success(t *testing.T) {
	router := setupAnalyticsRouterWithOperations(t, &stubAPIOperationsRepo{
		GetWebhookDeliveryTimeSeriesFunc: func(_ context.Context, _ string, _, _ time.Time, _, _ string) (*domain.WebhookDeliveryTimeSeriesResult, error) {
			return &domain.WebhookDeliveryTimeSeriesResult{
				Status:      "ready",
				WorkspaceID: "ws-1",
				Buckets: []domain.WebhookDeliveryTimeSeriesBucket{
					{BucketStart: "2025-01-15T00:00:00Z", Status: "success", Count: 100},
				},
			}, nil
		},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/ws-1/analytics/webhooks/delivery-timeseries", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var env analyticsEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	var result webhookDeliveryTimeSeriesResponseDoc
	if err := json.Unmarshal(env.Data, &result); err != nil {
		t.Fatalf("failed to unmarshal data: %v", err)
	}
	if result.Status != "ready" {
		t.Fatalf("expected status ready, got %s", result.Status)
	}
	if len(result.Buckets) != 1 {
		t.Fatalf("expected 1 bucket, got %d", len(result.Buckets))
	}
}

func TestAnalyticsGetWebhookDeliveryTimeSeries_InvalidInterval(t *testing.T) {
	router := setupAnalyticsRouterWithOperations(t, &stubAPIOperationsRepo{})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/ws-1/analytics/webhooks/delivery-timeseries?interval=month", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Operations: GetWebhookReliability
// ---------------------------------------------------------------------------

func TestAnalyticsGetWebhookReliability_Success(t *testing.T) {
	router := setupAnalyticsRouterWithOperations(t, &stubAPIOperationsRepo{
		GetWebhookReliabilityFunc: func(_ context.Context, _ string, _, _ time.Time, _ string) (*domain.WebhookReliabilityResult, error) {
			return &domain.WebhookReliabilityResult{
				Status:      "ready",
				WorkspaceID: "ws-1",
				Rows: []domain.WebhookReliabilityRow{
					{Source: "webhooks", Target: "https://example.com/hook", TotalCount: 100, Succeeded: 95, Failed: 3, Retried: 2, SuccessRate: 95},
				},
			}, nil
		},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/ws-1/analytics/webhooks/reliability", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var env analyticsEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	var result webhookReliabilityResponseDoc
	if err := json.Unmarshal(env.Data, &result); err != nil {
		t.Fatalf("failed to unmarshal data: %v", err)
	}
	if result.Status != "ready" {
		t.Fatalf("expected status ready, got %s", result.Status)
	}
	if len(result.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(result.Rows))
	}
}

func TestAnalyticsGetCampaignEvents_Pagination(t *testing.T) {
	router := setupAnalyticsRouter(t, &stubCampaignQueryRepo{
		GetCampaignEventsFunc: func(_ context.Context, filter domain.CampaignQueryFilter) (*domain.CampaignEventsResult, error) {
			return &domain.CampaignEventsResult{
				Status:      "ready",
				WorkspaceID: "ws-1",
				CampaignID:  "camp-1",
				Events: []domain.CampaignEventRow{
					{SourceEventID: "evt-1", EventType: "delivered", OccurredAt: "2025-01-15T10:00:00Z", ReceivedAt: "2025-01-15T10:00:01Z"},
					{SourceEventID: "evt-2", EventType: "opened", OccurredAt: "2025-01-15T10:05:00Z", ReceivedAt: "2025-01-15T10:05:01Z"},
				},
				NextCursor: "next-page-cursor",
			}, nil
		},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/ws-1/analytics/campaigns/camp-1/events?limit=2", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var env analyticsEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	var result campaignEventsResponseDoc
	if err := json.Unmarshal(env.Data, &result); err != nil {
		t.Fatalf("failed to unmarshal data: %v", err)
	}
	if result.NextCursor != "next-page-cursor" {
		t.Fatalf("expected next-page-cursor, got %s", result.NextCursor)
	}
	if len(result.Events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(result.Events))
	}
}
