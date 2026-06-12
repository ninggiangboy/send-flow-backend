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
	GetCampaignFunnelFunc      func(ctx context.Context, workspaceID, campaignID string, from, to time.Time) (*domain.CampaignFunnel, error)
	GetCampaignTimeSeriesFunc  func(ctx context.Context, workspaceID, campaignID string, from, to time.Time, interval, eventType string) (*domain.CampaignTimeSeriesResult, error)
	GetCampaignBreakdownFunc   func(ctx context.Context, workspaceID, campaignID string, from, to time.Time, groupBy string) (*domain.CampaignBreakdownResult, error)
	GetCampaignEventsFunc      func(ctx context.Context, filter domain.CampaignQueryFilter) (*domain.CampaignEventsResult, error)
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
				Status:        "ready",
				WorkspaceID:   "ws-1",
				CampaignID:    "camp-1",
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
