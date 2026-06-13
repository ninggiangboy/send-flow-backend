package app

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/contracts"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/ports"
)

// ---------------------------------------------------------------------------
// Stubs
// ---------------------------------------------------------------------------

type stubFactRepo struct {
	CreateFunc              func(ctx context.Context, fact domain.EmailEventFact) error
	FindBySourceEventIDFunc func(ctx context.Context, sourceEventID string) (*domain.EmailEventFact, error)
}

func (s *stubFactRepo) Create(ctx context.Context, fact domain.EmailEventFact) error {
	return s.CreateFunc(ctx, fact)
}
func (s *stubFactRepo) FindBySourceEventID(ctx context.Context, sourceEventID string) (*domain.EmailEventFact, error) {
	return s.FindBySourceEventIDFunc(ctx, sourceEventID)
}

type stubProjectionRead struct {
	GetWorkspaceOverviewFunc func(ctx context.Context, workspaceID string) (*domain.WorkspaceAnalyticsOverview, error)
	GetCampaignSummaryFunc   func(ctx context.Context, workspaceID, campaignID string) (*domain.CampaignDeliverySummary, error)
	ListDeliverabilityFunc   func(ctx context.Context, workspaceID string, filter domain.DeliverabilityFilter) ([]domain.DeliverabilityProjection, error)
}

func (s *stubProjectionRead) GetWorkspaceOverview(ctx context.Context, workspaceID string) (*domain.WorkspaceAnalyticsOverview, error) {
	return s.GetWorkspaceOverviewFunc(ctx, workspaceID)
}
func (s *stubProjectionRead) GetCampaignSummary(ctx context.Context, workspaceID, campaignID string) (*domain.CampaignDeliverySummary, error) {
	return s.GetCampaignSummaryFunc(ctx, workspaceID, campaignID)
}
func (s *stubProjectionRead) ListDeliverability(ctx context.Context, workspaceID string, filter domain.DeliverabilityFilter) ([]domain.DeliverabilityProjection, error) {
	return s.ListDeliverabilityFunc(ctx, workspaceID, filter)
}

type stubProjectionWrite struct {
	IncrementWorkspaceOverviewFunc func(ctx context.Context, workspaceID, eventType string, occurredAt time.Time) error
	IncrementCampaignSummaryFunc   func(ctx context.Context, workspaceID, campaignID, eventType string, occurredAt time.Time) error
	IncrementDeliverabilityFunc    func(ctx context.Context, workspaceID, provider, recipientDomain, eventType string, occurredAt time.Time) error
}

func (s *stubProjectionWrite) IncrementWorkspaceOverview(ctx context.Context, workspaceID, eventType string, occurredAt time.Time) error {
	return s.IncrementWorkspaceOverviewFunc(ctx, workspaceID, eventType, occurredAt)
}
func (s *stubProjectionWrite) IncrementCampaignSummary(ctx context.Context, workspaceID, campaignID, eventType string, occurredAt time.Time) error {
	return s.IncrementCampaignSummaryFunc(ctx, workspaceID, campaignID, eventType, occurredAt)
}
func (s *stubProjectionWrite) IncrementDeliverability(ctx context.Context, workspaceID, provider, recipientDomain, eventType string, occurredAt time.Time) error {
	return s.IncrementDeliverabilityFunc(ctx, workspaceID, provider, recipientDomain, eventType, occurredAt)
}

type stubTxManager struct {
	WithinTxFunc func(ctx context.Context, fn func(context.Context) error) error
}

func (s *stubTxManager) WithinTx(ctx context.Context, fn func(context.Context) error) error {
	return s.WithinTxFunc(ctx, fn)
}

type stubOutboxWriter struct {
	SaveFunc func(ctx context.Context, event ports.OutboxEvent) error
}

func (s *stubOutboxWriter) Save(ctx context.Context, event ports.OutboxEvent) error {
	return s.SaveFunc(ctx, event)
}

type stubAccessChecker struct {
	RequirePermissionFunc func(ctx context.Context, workspaceID, userID, permission string) error
}

func (s *stubAccessChecker) RequirePermission(ctx context.Context, workspaceID, userID, permission string) error {
	return s.RequirePermissionFunc(ctx, workspaceID, userID, permission)
}

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

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

var fixedTime = time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)

func newTestService(
	factRepo ports.EventFactRepository,
	projRead ports.ProjectionReadRepository,
	projWrite ports.ProjectionWriteRepository,
	txMgr ports.TransactionManager,
	outbox ports.OutboxWriter,
	checker ports.WorkspaceAccessChecker,
) *Service {
	return NewService(Options{
		FactRepo:        factRepo,
		ProjectionRead:  projRead,
		ProjectionWrite: projWrite,
		TxManager:       txMgr,
		OutboxWriter:    outbox,
		AccessChecker:   checker,
		IDGen:           func() (string, error) { return "fact-1", nil },
		Clock:           func() time.Time { return fixedTime },
		Logger:          slog.Default(),
	})
}

func newTestServiceWithQuery(
	factRepo ports.EventFactRepository,
	projRead ports.ProjectionReadRepository,
	projWrite ports.ProjectionWriteRepository,
	campaignQuery ports.CampaignQueryRepository,
	txMgr ports.TransactionManager,
	outbox ports.OutboxWriter,
	checker ports.WorkspaceAccessChecker,
) *Service {
	s := newTestService(factRepo, projRead, projWrite, txMgr, outbox, checker)
	s.campaignQueryRepo = campaignQuery
	return s
}

// ---------------------------------------------------------------------------
// IngestEmailEventFact
// ---------------------------------------------------------------------------

func TestIngestEmailEventFact_Success(t *testing.T) {
	var createdFact domain.EmailEventFact
	var savedOutbox *ports.OutboxEvent

	svc := newTestService(
		&stubFactRepo{
			FindBySourceEventIDFunc: func(_ context.Context, _ string) (*domain.EmailEventFact, error) {
				return nil, domain.ErrAnalyticsProjectionNotFound
			},
			CreateFunc: func(_ context.Context, fact domain.EmailEventFact) error {
				createdFact = fact
				return nil
			},
		},
		&stubProjectionRead{},
		&stubProjectionWrite{
			IncrementWorkspaceOverviewFunc: func(_ context.Context, _, _ string, _ time.Time) error { return nil },
			IncrementCampaignSummaryFunc:   func(_ context.Context, _, _, _ string, _ time.Time) error { return nil },
			IncrementDeliverabilityFunc:    func(_ context.Context, _, _, _, _ string, _ time.Time) error { return nil },
		},
		&stubTxManager{
			WithinTxFunc: func(_ context.Context, fn func(context.Context) error) error { return fn(context.Background()) },
		},
		&stubOutboxWriter{
			SaveFunc: func(_ context.Context, event ports.OutboxEvent) error {
				savedOutbox = &event
				return nil
			},
		},
		nil,
	)

	occurredAt := time.Date(2025, 1, 15, 9, 0, 0, 0, time.UTC)
	err := svc.IngestEmailEventFact(context.Background(), IngestEmailEventFactInput{
		SourceEventID:     "src-1",
		SourceEventType:   "test.event",
		WorkspaceID:       "ws-1",
		CampaignID:        "camp-1",
		MessageID:         "msg-1",
		Provider:          "sendgrid",
		ProviderMessageID: "sg-msg-1",
		ProviderEventID:   "sg-ev-1",
		CanonicalType:     domain.EventTypeDelivered,
		RecipientDomain:   "example.com",
		OccurredAt:        occurredAt,
		ReceivedAt:        fixedTime,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if createdFact.ID != "fact-1" {
		t.Fatalf("expected fact-1, got %s", createdFact.ID)
	}
	if createdFact.SourceEventID != "src-1" {
		t.Fatalf("expected src-1, got %s", createdFact.SourceEventID)
	}
	if createdFact.CreatedAt != fixedTime {
		t.Fatalf("expected fixed time, got %v", createdFact.CreatedAt)
	}
	if createdFact.CampaignID != "camp-1" {
		t.Fatalf("expected camp-1, got %s", createdFact.CampaignID)
	}

	if savedOutbox == nil {
		t.Fatal("expected outbox event to be saved")
	}
	if savedOutbox.EventType != contracts.EventProjectionUpdatedV1 {
		t.Fatalf("expected %s, got %s", contracts.EventProjectionUpdatedV1, savedOutbox.EventType)
	}
}

func TestIngestEmailEventFact_Deduplication(t *testing.T) {
	existing := &domain.EmailEventFact{SourceEventID: "src-1", WorkspaceID: "ws-1"}
	svc := newTestService(
		&stubFactRepo{
			FindBySourceEventIDFunc: func(_ context.Context, _ string) (*domain.EmailEventFact, error) {
				return existing, nil
			},
		},
		&stubProjectionRead{},
		&stubProjectionWrite{},
		&stubTxManager{
			WithinTxFunc: func(_ context.Context, fn func(context.Context) error) error { return fn(context.Background()) },
		},
		nil,
		nil,
	)

	err := svc.IngestEmailEventFact(context.Background(), IngestEmailEventFactInput{
		SourceEventID: "src-1",
		WorkspaceID:   "ws-1",
		CanonicalType: domain.EventTypeDelivered,
		OccurredAt:    fixedTime,
		ReceivedAt:    fixedTime,
	})
	if err != nil {
		t.Fatalf("expected nil for duplicate, got %v", err)
	}
}

func TestIngestEmailEventFact_ValidationError(t *testing.T) {
	svc := newTestService(nil, nil, nil, nil, nil, nil)
	err := svc.IngestEmailEventFact(context.Background(), IngestEmailEventFactInput{})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestIngestEmailEventFact_TransactionError(t *testing.T) {
	txErr := errors.New("tx failed")
	svc := newTestService(
		&stubFactRepo{},
		&stubProjectionRead{},
		&stubProjectionWrite{},
		&stubTxManager{
			WithinTxFunc: func(_ context.Context, _ func(context.Context) error) error { return txErr },
		},
		nil,
		nil,
	)

	err := svc.IngestEmailEventFact(context.Background(), IngestEmailEventFactInput{
		SourceEventID: "src-1",
		WorkspaceID:   "ws-1",
		CanonicalType: domain.EventTypeDelivered,
		OccurredAt:    fixedTime,
		ReceivedAt:    fixedTime,
	})
	if err != txErr {
		t.Fatalf("expected tx error, got %v", err)
	}
}

func TestIngestEmailEventFact_DuplicateRace(t *testing.T) {
	svc := newTestService(
		&stubFactRepo{
			FindBySourceEventIDFunc: func(_ context.Context, _ string) (*domain.EmailEventFact, error) {
				return nil, domain.ErrAnalyticsProjectionNotFound
			},
			CreateFunc: func(_ context.Context, _ domain.EmailEventFact) error {
				return domain.ErrAnalyticsEventDuplicate
			},
		},
		&stubProjectionRead{},
		&stubProjectionWrite{
			IncrementWorkspaceOverviewFunc: func(_ context.Context, _, _ string, _ time.Time) error { return nil },
		},
		&stubTxManager{
			WithinTxFunc: func(_ context.Context, fn func(context.Context) error) error { return fn(context.Background()) },
		},
		nil,
		nil,
	)

	err := svc.IngestEmailEventFact(context.Background(), IngestEmailEventFactInput{
		SourceEventID: "src-1",
		WorkspaceID:   "ws-1",
		CanonicalType: domain.EventTypeDelivered,
		OccurredAt:    fixedTime,
		ReceivedAt:    fixedTime,
	})
	if err != nil {
		t.Fatalf("expected nil on race duplicate, got %v", err)
	}
}

func TestIngestEmailEventFact_NoCampaignOrProvider(t *testing.T) {
	var incrementCampaignCalled, incrementDeliverabilityCalled bool

	svc := newTestService(
		&stubFactRepo{
			FindBySourceEventIDFunc: func(_ context.Context, _ string) (*domain.EmailEventFact, error) {
				return nil, domain.ErrAnalyticsProjectionNotFound
			},
			CreateFunc: func(_ context.Context, _ domain.EmailEventFact) error { return nil },
		},
		&stubProjectionRead{},
		&stubProjectionWrite{
			IncrementWorkspaceOverviewFunc: func(_ context.Context, _, _ string, _ time.Time) error { return nil },
			IncrementCampaignSummaryFunc: func(_ context.Context, _, _, _ string, _ time.Time) error {
				incrementCampaignCalled = true
				return nil
			},
			IncrementDeliverabilityFunc: func(_ context.Context, _, _, _, _ string, _ time.Time) error {
				incrementDeliverabilityCalled = true
				return nil
			},
		},
		&stubTxManager{
			WithinTxFunc: func(_ context.Context, fn func(context.Context) error) error { return fn(context.Background()) },
		},
		nil,
		nil,
	)

	err := svc.IngestEmailEventFact(context.Background(), IngestEmailEventFactInput{
		SourceEventID: "src-1",
		WorkspaceID:   "ws-1",
		CanonicalType: domain.EventTypeDelivered,
		OccurredAt:    fixedTime,
		ReceivedAt:    fixedTime,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if incrementCampaignCalled {
		t.Fatal("IncrementCampaignSummary should not have been called when CampaignID is empty")
	}
	if incrementDeliverabilityCalled {
		t.Fatal("IncrementDeliverability should not have been called when provider and domain are empty")
	}
}

// ---------------------------------------------------------------------------
// GetDashboardOverview
// ---------------------------------------------------------------------------

func TestGetDashboardOverview_Ready(t *testing.T) {
	lastEventAt := time.Date(2025, 1, 15, 8, 0, 0, 0, time.UTC)
	svc := newTestService(
		nil,
		&stubProjectionRead{
			GetWorkspaceOverviewFunc: func(_ context.Context, _ string) (*domain.WorkspaceAnalyticsOverview, error) {
				return &domain.WorkspaceAnalyticsOverview{
					WorkspaceID:     "ws-1",
					DeliveredCount:  100,
					BouncedCount:    5,
					ComplainedCount: 2,
					LastEventAt:     &lastEventAt,
					LastUpdatedAt:   fixedTime,
				}, nil
			},
		},
		nil, nil, nil, nil,
	)

	result, err := svc.GetDashboardOverview(context.Background(), GetDashboardOverviewInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "ready" {
		t.Fatalf("expected status ready, got %s", result.Status)
	}
	if result.DeliveredCount != 100 {
		t.Fatalf("expected 100 delivered, got %d", result.DeliveredCount)
	}
}

func TestGetDashboardOverview_Pending(t *testing.T) {
	svc := newTestService(
		nil,
		&stubProjectionRead{
			GetWorkspaceOverviewFunc: func(_ context.Context, _ string) (*domain.WorkspaceAnalyticsOverview, error) {
				return nil, domain.ErrAnalyticsProjectionNotFound
			},
		},
		nil, nil, nil, nil,
	)

	result, err := svc.GetDashboardOverview(context.Background(), GetDashboardOverviewInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "pending" {
		t.Fatalf("expected status pending, got %s", result.Status)
	}
	if result.WorkspaceID != "ws-1" {
		t.Fatalf("expected ws-1, got %s", result.WorkspaceID)
	}
}

func TestGetDashboardOverview_AccessDenied(t *testing.T) {
	svc := newTestService(
		nil, nil, nil, nil, nil,
		&stubAccessChecker{
			RequirePermissionFunc: func(_ context.Context, _, _, _ string) error {
				return errors.New("permission denied")
			},
		},
	)

	_, err := svc.GetDashboardOverview(context.Background(), GetDashboardOverviewInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
	})
	if err == nil {
		t.Fatal("expected access error")
	}
}

func TestGetDashboardOverview_RepoError(t *testing.T) {
	repoErr := errors.New("db gone")
	svc := newTestService(
		nil,
		&stubProjectionRead{
			GetWorkspaceOverviewFunc: func(_ context.Context, _ string) (*domain.WorkspaceAnalyticsOverview, error) {
				return nil, repoErr
			},
		},
		nil, nil, nil, nil,
	)

	_, err := svc.GetDashboardOverview(context.Background(), GetDashboardOverviewInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
	})
	if err != repoErr {
		t.Fatalf("expected repo error, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// GetCampaignAnalytics
// ---------------------------------------------------------------------------

func TestGetCampaignAnalytics_Success(t *testing.T) {
	lastEventAt := time.Date(2025, 1, 15, 8, 0, 0, 0, time.UTC)
	svc := newTestService(
		nil,
		&stubProjectionRead{
			GetCampaignSummaryFunc: func(_ context.Context, _, _ string) (*domain.CampaignDeliverySummary, error) {
				return &domain.CampaignDeliverySummary{
					WorkspaceID:     "ws-1",
					CampaignID:      "camp-1",
					DeliveredCount:  200,
					AcceptedCount:   250,
					BouncedCount:    10,
					ComplainedCount: 3,
					OpenedCount:     80,
					ClickedCount:    40,
					LastEventAt:     &lastEventAt,
					LastUpdatedAt:   fixedTime,
				}, nil
			},
		},
		nil, nil, nil, nil,
	)

	result, err := svc.GetCampaignAnalytics(context.Background(), GetCampaignAnalyticsInput{
		WorkspaceID: "ws-1",
		CampaignID:  "camp-1",
		UserID:      "user-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "ready" {
		t.Fatalf("expected status ready, got %s", result.Status)
	}
	if result.CampaignID != "camp-1" {
		t.Fatalf("expected camp-1, got %s", result.CampaignID)
	}
	if result.DeliveryRate != 80.0 {
		t.Fatalf("expected delivery rate 80, got %f", result.DeliveryRate)
	}
	if result.BounceRate != 5.0 {
		t.Fatalf("expected bounce rate 5, got %f", result.BounceRate)
	}
}

func TestGetCampaignAnalytics_AccessDenied(t *testing.T) {
	svc := newTestService(
		nil, nil, nil, nil, nil,
		&stubAccessChecker{
			RequirePermissionFunc: func(_ context.Context, _, _, _ string) error {
				return errors.New("permission denied")
			},
		},
	)

	_, err := svc.GetCampaignAnalytics(context.Background(), GetCampaignAnalyticsInput{
		WorkspaceID: "ws-1",
		CampaignID:  "camp-1",
		UserID:      "user-1",
	})
	if err == nil {
		t.Fatal("expected access error")
	}
}

func TestGetCampaignAnalytics_Pending(t *testing.T) {
	svc := newTestService(
		nil,
		&stubProjectionRead{
			GetCampaignSummaryFunc: func(_ context.Context, _, _ string) (*domain.CampaignDeliverySummary, error) {
				return nil, domain.ErrAnalyticsProjectionNotFound
			},
		},
		nil, nil, nil, nil,
	)

	result, err := svc.GetCampaignAnalytics(context.Background(), GetCampaignAnalyticsInput{
		WorkspaceID: "ws-1",
		CampaignID:  "camp-1",
		UserID:      "user-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "pending" {
		t.Fatalf("expected status pending, got %s", result.Status)
	}
}

func TestGetCampaignAnalytics_RepoError(t *testing.T) {
	repoErr := errors.New("connection lost")
	svc := newTestService(
		nil,
		&stubProjectionRead{
			GetCampaignSummaryFunc: func(_ context.Context, _, _ string) (*domain.CampaignDeliverySummary, error) {
				return nil, repoErr
			},
		},
		nil, nil, nil, nil,
	)

	_, err := svc.GetCampaignAnalytics(context.Background(), GetCampaignAnalyticsInput{
		WorkspaceID: "ws-1",
		CampaignID:  "camp-1",
		UserID:      "user-1",
	})
	if err != repoErr {
		t.Fatalf("expected repo error, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// GetDeliverability
// ---------------------------------------------------------------------------

func TestGetDeliverability_Success(t *testing.T) {
	lastEventAt := time.Date(2025, 1, 15, 8, 0, 0, 0, time.UTC)
	svc := newTestService(
		nil,
		&stubProjectionRead{
			ListDeliverabilityFunc: func(_ context.Context, _ string, _ domain.DeliverabilityFilter) ([]domain.DeliverabilityProjection, error) {
				return []domain.DeliverabilityProjection{
					{
						Provider:        "sendgrid",
						RecipientDomain: "example.com",
						DeliveredCount:  100,
						BouncedCount:    3,
						ComplainedCount: 1,
						OpenedCount:     40,
						ClickedCount:    20,
						LastEventAt:     &lastEventAt,
						LastUpdatedAt:   fixedTime,
					},
				}, nil
			},
		},
		nil, nil, nil, nil,
	)

	result, err := svc.GetDeliverability(context.Background(), GetDeliverabilityInput{
		WorkspaceID:     "ws-1",
		UserID:          "user-1",
		Provider:        "sendgrid",
		RecipientDomain: "example.com",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "ready" {
		t.Fatalf("expected status ready, got %s", result.Status)
	}
	if len(result.Items) != 1 {
		t.Fatalf("expected 1 row, got %d", len(result.Items))
	}
	if result.Items[0].BounceRate != 3.0 {
		t.Fatalf("expected bounce rate 3, got %f", result.Items[0].BounceRate)
	}
	if result.Items[0].ComplaintRate != 1.0 {
		t.Fatalf("expected complaint rate 1, got %f", result.Items[0].ComplaintRate)
	}
}

func TestGetDeliverability_Empty(t *testing.T) {
	svc := newTestService(
		nil,
		&stubProjectionRead{
			ListDeliverabilityFunc: func(_ context.Context, _ string, _ domain.DeliverabilityFilter) ([]domain.DeliverabilityProjection, error) {
				return nil, nil
			},
		},
		nil, nil, nil, nil,
	)

	result, err := svc.GetDeliverability(context.Background(), GetDeliverabilityInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "pending" {
		t.Fatalf("expected status pending, got %s", result.Status)
	}
	if result.Items != nil {
		t.Fatal("expected nil items")
	}
}

func TestGetDeliverability_FilterNormalization(t *testing.T) {
	var capturedFilter domain.DeliverabilityFilter

	svc := newTestService(
		nil,
		&stubProjectionRead{
			ListDeliverabilityFunc: func(_ context.Context, _ string, filter domain.DeliverabilityFilter) ([]domain.DeliverabilityProjection, error) {
				capturedFilter = filter
				return []domain.DeliverabilityProjection{
					{Provider: "ses", RecipientDomain: "test.com", DeliveredCount: 1},
				}, nil
			},
		},
		nil, nil, nil, nil,
	)

	_, err := svc.GetDeliverability(context.Background(), GetDeliverabilityInput{
		WorkspaceID:     "ws-1",
		UserID:          "user-1",
		Provider:        "  SES  ",
		RecipientDomain: "  TEST.COM  ",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedFilter.Provider != "ses" {
		t.Fatalf("expected normalized 'ses', got %q", capturedFilter.Provider)
	}
	if capturedFilter.RecipientDomain != "test.com" {
		t.Fatalf("expected normalized 'test.com', got %q", capturedFilter.RecipientDomain)
	}
}

func TestGetDeliverability_AccessDenied(t *testing.T) {
	svc := newTestService(
		nil, nil, nil, nil, nil,
		&stubAccessChecker{
			RequirePermissionFunc: func(_ context.Context, _, _, _ string) error {
				return errors.New("permission denied")
			},
		},
	)

	_, err := svc.GetDeliverability(context.Background(), GetDeliverabilityInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
	})
	if err == nil {
		t.Fatal("expected access error")
	}
}

func TestGetDeliverability_RepoError(t *testing.T) {
	repoErr := errors.New("repo down")
	svc := newTestService(
		nil,
		&stubProjectionRead{
			ListDeliverabilityFunc: func(_ context.Context, _ string, _ domain.DeliverabilityFilter) ([]domain.DeliverabilityProjection, error) {
				return nil, repoErr
			},
		},
		nil, nil, nil, nil,
	)

	_, err := svc.GetDeliverability(context.Background(), GetDeliverabilityInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
	})
	if err != repoErr {
		t.Fatalf("expected repo error, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// GetCampaignFunnel
// ---------------------------------------------------------------------------

func TestGetCampaignFunnel_Success(t *testing.T) {
	svc := newTestServiceWithQuery(
		nil, nil, nil,
		&stubCampaignQueryRepo{
			GetCampaignFunnelFunc: func(_ context.Context, _, _ string, _, _ time.Time) (*domain.CampaignFunnel, error) {
				return &domain.CampaignFunnel{
					Status:         "ready",
					WorkspaceID:    "ws-1",
					CampaignID:     "camp-1",
					DeliveredCount: 100,
				}, nil
			},
		},
		nil, nil, nil,
	)

	result, err := svc.GetCampaignFunnel(context.Background(), GetCampaignFunnelInput{
		WorkspaceID: "ws-1",
		CampaignID:  "camp-1",
		UserID:      "user-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "ready" {
		t.Fatalf("expected status ready, got %s", result.Status)
	}
	if result.DeliveredCount != 100 {
		t.Fatalf("expected 100 delivered, got %d", result.DeliveredCount)
	}
}

func TestGetCampaignFunnel_AccessDenied(t *testing.T) {
	svc := newTestServiceWithQuery(
		nil, nil, nil, nil, nil, nil,
		&stubAccessChecker{
			RequirePermissionFunc: func(_ context.Context, _, _, _ string) error {
				return errors.New("permission denied")
			},
		},
	)

	_, err := svc.GetCampaignFunnel(context.Background(), GetCampaignFunnelInput{
		WorkspaceID: "ws-1",
		CampaignID:  "camp-1",
		UserID:      "user-1",
	})
	if err == nil {
		t.Fatal("expected access error")
	}
}

func TestGetCampaignFunnel_NilRepo(t *testing.T) {
	svc := newTestService(nil, nil, nil, nil, nil, nil)

	_, err := svc.GetCampaignFunnel(context.Background(), GetCampaignFunnelInput{
		WorkspaceID: "ws-1",
		CampaignID:  "camp-1",
		UserID:      "user-1",
	})
	if err != domain.ErrAnalyticsStoreUnavailable {
		t.Fatalf("expected ErrAnalyticsQueryInvalid, got %v", err)
	}
}

func TestGetCampaignFunnel_InvalidTimeRange(t *testing.T) {
	svc := newTestServiceWithQuery(
		nil, nil, nil,
		&stubCampaignQueryRepo{},
		nil, nil, nil,
	)

	from := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)
	to := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	_, err := svc.GetCampaignFunnel(context.Background(), GetCampaignFunnelInput{
		WorkspaceID: "ws-1",
		CampaignID:  "camp-1",
		UserID:      "user-1",
		From:        from,
		To:          to,
	})
	if err == nil {
		t.Fatal("expected time range error")
	}
}

func TestGetCampaignFunnel_RepoError(t *testing.T) {
	repoErr := errors.New("clickhouse down")
	svc := newTestServiceWithQuery(
		nil, nil, nil,
		&stubCampaignQueryRepo{
			GetCampaignFunnelFunc: func(_ context.Context, _, _ string, _, _ time.Time) (*domain.CampaignFunnel, error) {
				return nil, repoErr
			},
		},
		nil, nil, nil,
	)

	_, err := svc.GetCampaignFunnel(context.Background(), GetCampaignFunnelInput{
		WorkspaceID: "ws-1",
		CampaignID:  "camp-1",
		UserID:      "user-1",
	})
	if err != repoErr {
		t.Fatalf("expected repo error, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// GetCampaignTimeSeries
// ---------------------------------------------------------------------------

func TestGetCampaignTimeSeries_Success(t *testing.T) {
	svc := newTestServiceWithQuery(
		nil, nil, nil,
		&stubCampaignQueryRepo{
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
		},
		nil, nil, nil,
	)

	result, err := svc.GetCampaignTimeSeries(context.Background(), GetCampaignTimeSeriesInput{
		WorkspaceID: "ws-1",
		CampaignID:  "camp-1",
		UserID:      "user-1",
		Interval:    "day",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "ready" {
		t.Fatalf("expected status ready, got %s", result.Status)
	}
	if len(result.Buckets) != 1 {
		t.Fatalf("expected 1 bucket, got %d", len(result.Buckets))
	}
}

func TestGetCampaignTimeSeries_InvalidInterval(t *testing.T) {
	svc := newTestServiceWithQuery(
		nil, nil, nil,
		&stubCampaignQueryRepo{},
		nil, nil, nil,
	)

	_, err := svc.GetCampaignTimeSeries(context.Background(), GetCampaignTimeSeriesInput{
		WorkspaceID: "ws-1",
		CampaignID:  "camp-1",
		UserID:      "user-1",
		Interval:    "month",
	})
	if err == nil {
		t.Fatal("expected error for invalid interval")
	}
}

func TestGetCampaignTimeSeries_UnknownEventType(t *testing.T) {
	svc := newTestServiceWithQuery(
		nil, nil, nil,
		&stubCampaignQueryRepo{},
		nil, nil, nil,
	)

	_, err := svc.GetCampaignTimeSeries(context.Background(), GetCampaignTimeSeriesInput{
		WorkspaceID: "ws-1",
		CampaignID:  "camp-1",
		UserID:      "user-1",
		Interval:    "day",
		EventType:   "unknown",
	})
	if err == nil {
		t.Fatal("expected error for unknown event_type")
	}
}

func TestGetCampaignTimeSeries_AccessDenied(t *testing.T) {
	svc := newTestServiceWithQuery(
		nil, nil, nil, nil, nil, nil,
		&stubAccessChecker{
			RequirePermissionFunc: func(_ context.Context, _, _, _ string) error {
				return errors.New("permission denied")
			},
		},
	)

	_, err := svc.GetCampaignTimeSeries(context.Background(), GetCampaignTimeSeriesInput{
		WorkspaceID: "ws-1",
		CampaignID:  "camp-1",
		UserID:      "user-1",
	})
	if err == nil {
		t.Fatal("expected access error")
	}
}

func TestGetCampaignTimeSeries_NilRepo(t *testing.T) {
	svc := newTestService(nil, nil, nil, nil, nil, nil)

	_, err := svc.GetCampaignTimeSeries(context.Background(), GetCampaignTimeSeriesInput{
		WorkspaceID: "ws-1",
		CampaignID:  "camp-1",
		UserID:      "user-1",
	})
	if err != domain.ErrAnalyticsStoreUnavailable {
		t.Fatalf("expected ErrAnalyticsQueryInvalid, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// GetCampaignBreakdown
// ---------------------------------------------------------------------------

func TestGetCampaignBreakdown_Success(t *testing.T) {
	svc := newTestServiceWithQuery(
		nil, nil, nil,
		&stubCampaignQueryRepo{
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
		},
		nil, nil, nil,
	)

	result, err := svc.GetCampaignBreakdown(context.Background(), GetCampaignBreakdownInput{
		WorkspaceID: "ws-1",
		CampaignID:  "camp-1",
		UserID:      "user-1",
		GroupBy:     "event_type",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "ready" {
		t.Fatalf("expected status ready, got %s", result.Status)
	}
	if len(result.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(result.Rows))
	}
}

func TestGetCampaignBreakdown_InvalidGroup(t *testing.T) {
	svc := newTestServiceWithQuery(
		nil, nil, nil,
		&stubCampaignQueryRepo{},
		nil, nil, nil,
	)

	_, err := svc.GetCampaignBreakdown(context.Background(), GetCampaignBreakdownInput{
		WorkspaceID: "ws-1",
		CampaignID:  "camp-1",
		UserID:      "user-1",
		GroupBy:     "unknown",
	})
	if err == nil {
		t.Fatal("expected error for invalid group_by")
	}
}

func TestGetCampaignBreakdown_AccessDenied(t *testing.T) {
	svc := newTestServiceWithQuery(
		nil, nil, nil, nil, nil, nil,
		&stubAccessChecker{
			RequirePermissionFunc: func(_ context.Context, _, _, _ string) error {
				return errors.New("permission denied")
			},
		},
	)

	_, err := svc.GetCampaignBreakdown(context.Background(), GetCampaignBreakdownInput{
		WorkspaceID: "ws-1",
		CampaignID:  "camp-1",
		UserID:      "user-1",
	})
	if err == nil {
		t.Fatal("expected access error")
	}
}

func TestGetCampaignBreakdown_NilRepo(t *testing.T) {
	svc := newTestService(nil, nil, nil, nil, nil, nil)

	_, err := svc.GetCampaignBreakdown(context.Background(), GetCampaignBreakdownInput{
		WorkspaceID: "ws-1",
		CampaignID:  "camp-1",
		UserID:      "user-1",
	})
	if err != domain.ErrAnalyticsStoreUnavailable {
		t.Fatalf("expected ErrAnalyticsQueryInvalid, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// GetCampaignEvents
// ---------------------------------------------------------------------------

func TestGetCampaignEvents_Success(t *testing.T) {
	svc := newTestServiceWithQuery(
		nil, nil, nil,
		&stubCampaignQueryRepo{
			GetCampaignEventsFunc: func(_ context.Context, _ domain.CampaignQueryFilter) (*domain.CampaignEventsResult, error) {
				return &domain.CampaignEventsResult{
					Status:      "ready",
					WorkspaceID: "ws-1",
					CampaignID:  "camp-1",
					Events: []domain.CampaignEventRow{
						{SourceEventID: "evt-1", EventType: "delivered"},
					},
				}, nil
			},
		},
		nil, nil, nil,
	)

	result, err := svc.GetCampaignEvents(context.Background(), GetCampaignEventsInput{
		WorkspaceID: "ws-1",
		CampaignID:  "camp-1",
		UserID:      "user-1",
		Limit:       50,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "ready" {
		t.Fatalf("expected status ready, got %s", result.Status)
	}
	if len(result.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(result.Events))
	}
}

func TestGetCampaignEvents_DefaultLimit(t *testing.T) {
	var capturedFilter domain.CampaignQueryFilter

	svc := newTestServiceWithQuery(
		nil, nil, nil,
		&stubCampaignQueryRepo{
			GetCampaignEventsFunc: func(_ context.Context, filter domain.CampaignQueryFilter) (*domain.CampaignEventsResult, error) {
				capturedFilter = filter
				return &domain.CampaignEventsResult{
					Status:      "ready",
					WorkspaceID: "ws-1",
					CampaignID:  "camp-1",
				}, nil
			},
		},
		nil, nil, nil,
	)

	_, err := svc.GetCampaignEvents(context.Background(), GetCampaignEventsInput{
		WorkspaceID: "ws-1",
		CampaignID:  "camp-1",
		UserID:      "user-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedFilter.Limit != 50 {
		t.Fatalf("expected default limit 50, got %d", capturedFilter.Limit)
	}
}

func TestGetCampaignEvents_AccessDenied(t *testing.T) {
	svc := newTestServiceWithQuery(
		nil, nil, nil, nil, nil, nil,
		&stubAccessChecker{
			RequirePermissionFunc: func(_ context.Context, _, _, _ string) error {
				return errors.New("permission denied")
			},
		},
	)

	_, err := svc.GetCampaignEvents(context.Background(), GetCampaignEventsInput{
		WorkspaceID: "ws-1",
		CampaignID:  "camp-1",
		UserID:      "user-1",
	})
	if err == nil {
		t.Fatal("expected access error")
	}
}

func TestGetCampaignEvents_NilRepo(t *testing.T) {
	svc := newTestService(nil, nil, nil, nil, nil, nil)

	_, err := svc.GetCampaignEvents(context.Background(), GetCampaignEventsInput{
		WorkspaceID: "ws-1",
		CampaignID:  "camp-1",
		UserID:      "user-1",
	})
	if err != domain.ErrAnalyticsStoreUnavailable {
		t.Fatalf("expected ErrAnalyticsQueryInvalid, got %v", err)
	}
}

func TestGetCampaignEvents_RepoError(t *testing.T) {
	repoErr := errors.New("query failed")
	svc := newTestServiceWithQuery(
		nil, nil, nil,
		&stubCampaignQueryRepo{
			GetCampaignEventsFunc: func(_ context.Context, _ domain.CampaignQueryFilter) (*domain.CampaignEventsResult, error) {
				return nil, repoErr
			},
		},
		nil, nil, nil,
	)

	_, err := svc.GetCampaignEvents(context.Background(), GetCampaignEventsInput{
		WorkspaceID: "ws-1",
		CampaignID:  "camp-1",
		UserID:      "user-1",
	})
	if err != repoErr {
		t.Fatalf("expected repo error, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Stub Deliverability Query Repo
// ---------------------------------------------------------------------------

type stubDeliverabilityQueryRepo struct {
	GetDeliverabilityTimeSeriesFunc func(ctx context.Context, workspaceID string, from, to time.Time, provider, recipientDomain, interval string) (*domain.DeliverabilityTimeSeriesResult, error)
	GetDeliverabilityBreakdownFunc  func(ctx context.Context, workspaceID string, from, to time.Time, groupBy, provider, recipientDomain string) (*domain.DeliverabilityBreakdownResult, error)
	GetDeliverabilityLatencyFunc    func(ctx context.Context, workspaceID string, from, to time.Time, provider, recipientDomain string) (*domain.DeliverabilityLatencyResult, error)
	GetDeliverabilityIncidentsFunc  func(ctx context.Context, workspaceID string, from, to time.Time, provider, recipientDomain string) (*domain.DeliverabilityIncidentResult, error)
}

func (s *stubDeliverabilityQueryRepo) GetDeliverabilityTimeSeries(ctx context.Context, workspaceID string, from, to time.Time, provider, recipientDomain, interval string) (*domain.DeliverabilityTimeSeriesResult, error) {
	return s.GetDeliverabilityTimeSeriesFunc(ctx, workspaceID, from, to, provider, recipientDomain, interval)
}
func (s *stubDeliverabilityQueryRepo) GetDeliverabilityBreakdown(ctx context.Context, workspaceID string, from, to time.Time, groupBy, provider, recipientDomain string) (*domain.DeliverabilityBreakdownResult, error) {
	return s.GetDeliverabilityBreakdownFunc(ctx, workspaceID, from, to, groupBy, provider, recipientDomain)
}
func (s *stubDeliverabilityQueryRepo) GetDeliverabilityLatency(ctx context.Context, workspaceID string, from, to time.Time, provider, recipientDomain string) (*domain.DeliverabilityLatencyResult, error) {
	return s.GetDeliverabilityLatencyFunc(ctx, workspaceID, from, to, provider, recipientDomain)
}
func (s *stubDeliverabilityQueryRepo) GetDeliverabilityIncidents(ctx context.Context, workspaceID string, from, to time.Time, provider, recipientDomain string) (*domain.DeliverabilityIncidentResult, error) {
	return s.GetDeliverabilityIncidentsFunc(ctx, workspaceID, from, to, provider, recipientDomain)
}

func newTestServiceWithDeliverabilityQuery(
	factRepo ports.EventFactRepository,
	projRead ports.ProjectionReadRepository,
	projWrite ports.ProjectionWriteRepository,
	deliverabilityQuery ports.DeliverabilityQueryRepository,
	txMgr ports.TransactionManager,
	outbox ports.OutboxWriter,
	checker ports.WorkspaceAccessChecker,
) *Service {
	s := newTestService(factRepo, projRead, projWrite, txMgr, outbox, checker)
	s.deliverabilityQueryRepo = deliverabilityQuery
	return s
}

// ---------------------------------------------------------------------------
// GetDeliverabilityTimeSeries
// ---------------------------------------------------------------------------

func TestGetDeliverabilityTimeSeries_Success(t *testing.T) {
	svc := newTestServiceWithDeliverabilityQuery(
		nil, nil, nil,
		&stubDeliverabilityQueryRepo{
			GetDeliverabilityTimeSeriesFunc: func(_ context.Context, workspaceID string, _, _ time.Time, _, _, interval string) (*domain.DeliverabilityTimeSeriesResult, error) {
				return &domain.DeliverabilityTimeSeriesResult{
					Status:      "ready",
					WorkspaceID: workspaceID,
					Buckets: []domain.DeliverabilityTimeSeriesBucket{
						{BucketStart: fixedTime, Provider: "sendgrid", EventType: "delivered", Count: 100},
					},
				}, nil
			},
		},
		nil, nil, nil,
	)

	result, err := svc.GetDeliverabilityTimeSeries(context.Background(), GetDeliverabilityTimeSeriesInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
		From:        fixedTime.Add(-24 * time.Hour),
		To:          fixedTime,
		Interval:    "day",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "ready" {
		t.Fatalf("expected ready, got %s", result.Status)
	}
	if len(result.Buckets) != 1 {
		t.Fatalf("expected 1 bucket, got %d", len(result.Buckets))
	}
}

func TestGetDeliverabilityTimeSeries_InvalidInterval(t *testing.T) {
	svc := newTestServiceWithDeliverabilityQuery(nil, nil, nil, nil, nil, nil, nil)

	_, err := svc.GetDeliverabilityTimeSeries(context.Background(), GetDeliverabilityTimeSeriesInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
		Interval:    "invalid",
	})
	if err == nil {
		t.Fatal("expected validation error for invalid interval")
	}
}

func TestGetDeliverabilityTimeSeries_InvalidTimeRange(t *testing.T) {
	svc := newTestServiceWithDeliverabilityQuery(nil, nil, nil, nil, nil, nil, nil)

	_, err := svc.GetDeliverabilityTimeSeries(context.Background(), GetDeliverabilityTimeSeriesInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
		From:        fixedTime,
		To:          fixedTime.Add(-24 * time.Hour),
	})
	if err == nil {
		t.Fatal("expected validation error for invalid time range")
	}
}

func TestGetDeliverabilityTimeSeries_AccessDenied(t *testing.T) {
	svc := newTestServiceWithDeliverabilityQuery(
		nil, nil, nil, nil, nil, nil,
		&stubAccessChecker{
			RequirePermissionFunc: func(_ context.Context, _, _, _ string) error {
				return errors.New("permission denied")
			},
		},
	)

	_, err := svc.GetDeliverabilityTimeSeries(context.Background(), GetDeliverabilityTimeSeriesInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
	})
	if err == nil {
		t.Fatal("expected access error")
	}
}

func TestGetDeliverabilityTimeSeries_NilRepo(t *testing.T) {
	svc := newTestService(nil, nil, nil, nil, nil, nil)

	_, err := svc.GetDeliverabilityTimeSeries(context.Background(), GetDeliverabilityTimeSeriesInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
	})
	if err != domain.ErrAnalyticsStoreUnavailable {
		t.Fatalf("expected ErrAnalyticsQueryInvalid, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// GetDeliverabilityBreakdown
// ---------------------------------------------------------------------------

func TestGetDeliverabilityBreakdown_Success(t *testing.T) {
	svc := newTestServiceWithDeliverabilityQuery(
		nil, nil, nil,
		&stubDeliverabilityQueryRepo{
			GetDeliverabilityBreakdownFunc: func(_ context.Context, workspaceID string, _, _ time.Time, groupBy, _, _ string) (*domain.DeliverabilityBreakdownResult, error) {
				return &domain.DeliverabilityBreakdownResult{
					Status:      "ready",
					WorkspaceID: workspaceID,
					GroupBy:     groupBy,
					Rows: []domain.DeliverabilityBreakdownRow{
						{Provider: "sendgrid", EventType: "delivered", Count: 100, Rate: 100},
					},
				}, nil
			},
		},
		nil, nil, nil,
	)

	result, err := svc.GetDeliverabilityBreakdown(context.Background(), GetDeliverabilityBreakdownInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
		GroupBy:     "provider",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "ready" {
		t.Fatalf("expected ready, got %s", result.Status)
	}
}

func TestGetDeliverabilityBreakdown_InvalidGroupBy(t *testing.T) {
	svc := newTestServiceWithDeliverabilityQuery(nil, nil, nil, nil, nil, nil, nil)

	_, err := svc.GetDeliverabilityBreakdown(context.Background(), GetDeliverabilityBreakdownInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
		GroupBy:     "event_type",
	})
	if err == nil {
		t.Fatal("expected validation error for event_type group_by")
	}
}

// ---------------------------------------------------------------------------
// GetDeliverabilityLatency
// ---------------------------------------------------------------------------

func TestGetDeliverabilityLatency_Success(t *testing.T) {
	svc := newTestServiceWithDeliverabilityQuery(
		nil, nil, nil,
		&stubDeliverabilityQueryRepo{
			GetDeliverabilityLatencyFunc: func(_ context.Context, workspaceID string, _, _ time.Time, _, _ string) (*domain.DeliverabilityLatencyResult, error) {
				return &domain.DeliverabilityLatencyResult{
					Status:      "ready",
					WorkspaceID: workspaceID,
					Rows: []domain.DeliverabilityLatencyRow{
						{Provider: "sendgrid", EventType: "accepted_to_delivered", Count: 100, P50LatencyMs: 500, P95LatencyMs: 2000, P99LatencyMs: 5000, AvgLatencyMs: 750},
					},
				}, nil
			},
		},
		nil, nil, nil,
	)

	result, err := svc.GetDeliverabilityLatency(context.Background(), GetDeliverabilityLatencyInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(result.Rows))
	}
}

// ---------------------------------------------------------------------------
// GetDeliverabilityIncidents
// ---------------------------------------------------------------------------

func TestGetDeliverabilityIncidents_Success(t *testing.T) {
	svc := newTestServiceWithDeliverabilityQuery(
		nil, nil, nil,
		&stubDeliverabilityQueryRepo{
			GetDeliverabilityIncidentsFunc: func(_ context.Context, workspaceID string, _, _ time.Time, _, _ string) (*domain.DeliverabilityIncidentResult, error) {
				return &domain.DeliverabilityIncidentResult{
					Status:      "ready",
					WorkspaceID: workspaceID,
					Rows: []domain.DeliverabilityIncidentRow{
						{Provider: "sendgrid", EventType: "bounce", IncidentStart: fixedTime.Format(time.RFC3339), EventCount: 10, Rate: 10.5},
					},
				}, nil
			},
		},
		nil, nil, nil,
	)

	result, err := svc.GetDeliverabilityIncidents(context.Background(), GetDeliverabilityIncidentsInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(result.Rows))
	}
}

func TestGetDeliverabilityIncidents_InvalidTimeRange(t *testing.T) {
	svc := newTestServiceWithDeliverabilityQuery(nil, nil, nil, nil, nil, nil, nil)

	_, err := svc.GetDeliverabilityIncidents(context.Background(), GetDeliverabilityIncidentsInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
		From:        fixedTime,
		To:          fixedTime.Add(-24 * time.Hour),
	})
	if err == nil {
		t.Fatal("expected validation error for invalid time range")
	}
}

// ---------------------------------------------------------------------------
// Stub Forensic Query Repo
// ---------------------------------------------------------------------------

type stubForensicQueryRepo struct {
	SearchEventsFunc                func(ctx context.Context, filter domain.ForensicQueryFilter) (*domain.ForensicEventsResult, error)
	GetMessageTimelineFunc          func(ctx context.Context, workspaceID, messageID string) (*domain.MessageTimelineResult, error)
	GetProviderEventTraceFunc       func(ctx context.Context, workspaceID, providerEventID string) (*domain.ProviderEventTrace, error)
	GetCampaignIncidentTimelineFunc func(ctx context.Context, workspaceID, campaignID string, from, to time.Time) (*domain.CampaignIncidentTimelineResult, error)
}

func (s *stubForensicQueryRepo) SearchEvents(ctx context.Context, filter domain.ForensicQueryFilter) (*domain.ForensicEventsResult, error) {
	return s.SearchEventsFunc(ctx, filter)
}
func (s *stubForensicQueryRepo) GetMessageTimeline(ctx context.Context, workspaceID, messageID string) (*domain.MessageTimelineResult, error) {
	return s.GetMessageTimelineFunc(ctx, workspaceID, messageID)
}
func (s *stubForensicQueryRepo) GetProviderEventTrace(ctx context.Context, workspaceID, providerEventID string) (*domain.ProviderEventTrace, error) {
	return s.GetProviderEventTraceFunc(ctx, workspaceID, providerEventID)
}
func (s *stubForensicQueryRepo) GetCampaignIncidentTimeline(ctx context.Context, workspaceID, campaignID string, from, to time.Time) (*domain.CampaignIncidentTimelineResult, error) {
	return s.GetCampaignIncidentTimelineFunc(ctx, workspaceID, campaignID, from, to)
}

func newTestServiceWithForensicQuery(
	factRepo ports.EventFactRepository,
	projRead ports.ProjectionReadRepository,
	projWrite ports.ProjectionWriteRepository,
	forensicQuery ports.ForensicQueryRepository,
	txMgr ports.TransactionManager,
	outbox ports.OutboxWriter,
	checker ports.WorkspaceAccessChecker,
) *Service {
	s := newTestService(factRepo, projRead, projWrite, txMgr, outbox, checker)
	s.forensicQueryRepo = forensicQuery
	return s
}

// ---------------------------------------------------------------------------
// SearchEvents
// ---------------------------------------------------------------------------

func TestSearchEvents_Success(t *testing.T) {
	svc := newTestServiceWithForensicQuery(
		nil, nil, nil,
		&stubForensicQueryRepo{
			SearchEventsFunc: func(_ context.Context, _ domain.ForensicQueryFilter) (*domain.ForensicEventsResult, error) {
				return &domain.ForensicEventsResult{
					Status:      "ready",
					WorkspaceID: "ws-1",
					Events: []domain.ForensicEventRow{
						{SourceEventID: "evt-1", EventType: "delivered", OccurredAt: "2025-01-15T10:00:00Z"},
					},
				}, nil
			},
		},
		nil, nil, nil,
	)

	result, err := svc.SearchEvents(context.Background(), SearchEventsInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
		From:        fixedTime.Add(-24 * time.Hour),
		To:          fixedTime,
		Limit:       50,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
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

func TestSearchEvents_AccessDenied(t *testing.T) {
	svc := newTestServiceWithForensicQuery(
		nil, nil, nil, nil, nil, nil,
		&stubAccessChecker{
			RequirePermissionFunc: func(_ context.Context, _, _, _ string) error {
				return errors.New("permission denied")
			},
		},
	)

	_, err := svc.SearchEvents(context.Background(), SearchEventsInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
	})
	if err == nil {
		t.Fatal("expected access error")
	}
}

func TestSearchEvents_NilRepo(t *testing.T) {
	svc := newTestService(nil, nil, nil, nil, nil, nil)

	_, err := svc.SearchEvents(context.Background(), SearchEventsInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
	})
	if err != domain.ErrAnalyticsStoreUnavailable {
		t.Fatalf("expected ErrAnalyticsQueryInvalid, got %v", err)
	}
}

func TestSearchEvents_ValidationError(t *testing.T) {
	svc := newTestServiceWithForensicQuery(nil, nil, nil, &stubForensicQueryRepo{}, nil, nil, nil)

	_, err := svc.SearchEvents(context.Background(), SearchEventsInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
	})
	if err == nil {
		t.Fatal("expected validation error for unbounded search")
	}
}

func TestSearchEvents_RepoError(t *testing.T) {
	repoErr := errors.New("clickhouse down")
	svc := newTestServiceWithForensicQuery(
		nil, nil, nil,
		&stubForensicQueryRepo{
			SearchEventsFunc: func(_ context.Context, _ domain.ForensicQueryFilter) (*domain.ForensicEventsResult, error) {
				return nil, repoErr
			},
		},
		nil, nil, nil,
	)

	_, err := svc.SearchEvents(context.Background(), SearchEventsInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
		From:        fixedTime.Add(-24 * time.Hour),
		To:          fixedTime,
		Limit:       50,
	})
	if err != repoErr {
		t.Fatalf("expected repo error, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// GetMessageTimeline
// ---------------------------------------------------------------------------

func TestGetMessageTimeline_Success(t *testing.T) {
	svc := newTestServiceWithForensicQuery(
		nil, nil, nil,
		&stubForensicQueryRepo{
			GetMessageTimelineFunc: func(_ context.Context, _, _ string) (*domain.MessageTimelineResult, error) {
				return &domain.MessageTimelineResult{
					Status:      "ready",
					WorkspaceID: "ws-1",
					MessageID:   "msg-1",
					Events: []domain.MessageTimelineRow{
						{SourceEventID: "evt-1", EventType: "accepted", OccurredAt: "2025-01-15T09:00:00Z"},
						{SourceEventID: "evt-2", EventType: "delivered", OccurredAt: "2025-01-15T09:01:00Z"},
					},
				}, nil
			},
		},
		nil, nil, nil,
	)

	result, err := svc.GetMessageTimeline(context.Background(), GetMessageTimelineInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
		MessageID:   "msg-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "ready" {
		t.Fatalf("expected status ready, got %s", result.Status)
	}
	if len(result.Events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(result.Events))
	}
	if result.Events[0].EventType != "accepted" {
		t.Fatalf("expected first event accepted, got %s", result.Events[0].EventType)
	}
}

func TestGetMessageTimeline_AccessDenied(t *testing.T) {
	svc := newTestServiceWithForensicQuery(
		nil, nil, nil, nil, nil, nil,
		&stubAccessChecker{
			RequirePermissionFunc: func(_ context.Context, _, _, _ string) error {
				return errors.New("permission denied")
			},
		},
	)

	_, err := svc.GetMessageTimeline(context.Background(), GetMessageTimelineInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
		MessageID:   "msg-1",
	})
	if err == nil {
		t.Fatal("expected access error")
	}
}

func TestGetMessageTimeline_NilRepo(t *testing.T) {
	svc := newTestService(nil, nil, nil, nil, nil, nil)

	_, err := svc.GetMessageTimeline(context.Background(), GetMessageTimelineInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
		MessageID:   "msg-1",
	})
	if err != domain.ErrAnalyticsStoreUnavailable {
		t.Fatalf("expected ErrAnalyticsQueryInvalid, got %v", err)
	}
}

func TestGetMessageTimeline_RepoError(t *testing.T) {
	repoErr := errors.New("clickhouse down")
	svc := newTestServiceWithForensicQuery(
		nil, nil, nil,
		&stubForensicQueryRepo{
			GetMessageTimelineFunc: func(_ context.Context, _, _ string) (*domain.MessageTimelineResult, error) {
				return nil, repoErr
			},
		},
		nil, nil, nil,
	)

	_, err := svc.GetMessageTimeline(context.Background(), GetMessageTimelineInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
		MessageID:   "msg-1",
	})
	if err != repoErr {
		t.Fatalf("expected repo error, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// GetProviderEventTrace
// ---------------------------------------------------------------------------

func TestGetProviderEventTrace_Success(t *testing.T) {
	svc := newTestServiceWithForensicQuery(
		nil, nil, nil,
		&stubForensicQueryRepo{
			GetProviderEventTraceFunc: func(_ context.Context, _, _ string) (*domain.ProviderEventTrace, error) {
				return &domain.ProviderEventTrace{
					Status:          "ready",
					ProviderEventID: "pe-1",
					WorkspaceID:     "ws-1",
					CampaignID:      "camp-1",
					MessageID:       "msg-1",
					Provider:        "sendgrid",
				}, nil
			},
		},
		nil, nil, nil,
	)

	result, err := svc.GetProviderEventTrace(context.Background(), GetProviderEventTraceInput{
		WorkspaceID:     "ws-1",
		UserID:          "user-1",
		ProviderEventID: "pe-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "ready" {
		t.Fatalf("expected status ready, got %s", result.Status)
	}
	if result.ProviderEventID != "pe-1" {
		t.Fatalf("expected pe-1, got %s", result.ProviderEventID)
	}
	if result.MessageID != "msg-1" {
		t.Fatalf("expected msg-1, got %s", result.MessageID)
	}
}

func TestGetProviderEventTrace_AccessDenied(t *testing.T) {
	svc := newTestServiceWithForensicQuery(
		nil, nil, nil, nil, nil, nil,
		&stubAccessChecker{
			RequirePermissionFunc: func(_ context.Context, _, _, _ string) error {
				return errors.New("permission denied")
			},
		},
	)

	_, err := svc.GetProviderEventTrace(context.Background(), GetProviderEventTraceInput{
		WorkspaceID:     "ws-1",
		UserID:          "user-1",
		ProviderEventID: "pe-1",
	})
	if err == nil {
		t.Fatal("expected access error")
	}
}

func TestGetProviderEventTrace_NilRepo(t *testing.T) {
	svc := newTestService(nil, nil, nil, nil, nil, nil)

	_, err := svc.GetProviderEventTrace(context.Background(), GetProviderEventTraceInput{
		WorkspaceID:     "ws-1",
		UserID:          "user-1",
		ProviderEventID: "pe-1",
	})
	if err != domain.ErrAnalyticsStoreUnavailable {
		t.Fatalf("expected ErrAnalyticsQueryInvalid, got %v", err)
	}
}

func TestGetProviderEventTrace_RepoError(t *testing.T) {
	repoErr := errors.New("clickhouse down")
	svc := newTestServiceWithForensicQuery(
		nil, nil, nil,
		&stubForensicQueryRepo{
			GetProviderEventTraceFunc: func(_ context.Context, _, _ string) (*domain.ProviderEventTrace, error) {
				return nil, repoErr
			},
		},
		nil, nil, nil,
	)

	_, err := svc.GetProviderEventTrace(context.Background(), GetProviderEventTraceInput{
		WorkspaceID:     "ws-1",
		UserID:          "user-1",
		ProviderEventID: "pe-1",
	})
	if err != repoErr {
		t.Fatalf("expected repo error, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// GetCampaignIncidentTimeline
// ---------------------------------------------------------------------------

func TestGetCampaignIncidentTimeline_Success(t *testing.T) {
	svc := newTestServiceWithForensicQuery(
		nil, nil, nil,
		&stubForensicQueryRepo{
			GetCampaignIncidentTimelineFunc: func(_ context.Context, _, _ string, _, _ time.Time) (*domain.CampaignIncidentTimelineResult, error) {
				return &domain.CampaignIncidentTimelineResult{
					Status:      "ready",
					WorkspaceID: "ws-1",
					CampaignID:  "camp-1",
					Events: []domain.CampaignIncidentTimelineRow{
						{SourceEventID: "evt-1", EventType: "bounced", MessageID: "msg-1"},
					},
				}, nil
			},
		},
		nil, nil, nil,
	)

	result, err := svc.GetCampaignIncidentTimeline(context.Background(), GetCampaignIncidentTimelineInput{
		WorkspaceID: "ws-1",
		CampaignID:  "camp-1",
		UserID:      "user-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "ready" {
		t.Fatalf("expected status ready, got %s", result.Status)
	}
	if len(result.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(result.Events))
	}
}

func TestGetCampaignIncidentTimeline_AccessDenied(t *testing.T) {
	svc := newTestServiceWithForensicQuery(
		nil, nil, nil, nil, nil, nil,
		&stubAccessChecker{
			RequirePermissionFunc: func(_ context.Context, _, _, _ string) error {
				return errors.New("permission denied")
			},
		},
	)

	_, err := svc.GetCampaignIncidentTimeline(context.Background(), GetCampaignIncidentTimelineInput{
		WorkspaceID: "ws-1",
		CampaignID:  "camp-1",
		UserID:      "user-1",
	})
	if err == nil {
		t.Fatal("expected access error")
	}
}

func TestGetCampaignIncidentTimeline_NilRepo(t *testing.T) {
	svc := newTestService(nil, nil, nil, nil, nil, nil)

	_, err := svc.GetCampaignIncidentTimeline(context.Background(), GetCampaignIncidentTimelineInput{
		WorkspaceID: "ws-1",
		CampaignID:  "camp-1",
		UserID:      "user-1",
	})
	if err != domain.ErrAnalyticsStoreUnavailable {
		t.Fatalf("expected ErrAnalyticsQueryInvalid, got %v", err)
	}
}

func TestGetCampaignIncidentTimeline_InvalidTimeRange(t *testing.T) {
	svc := newTestServiceWithForensicQuery(nil, nil, nil, &stubForensicQueryRepo{}, nil, nil, nil)

	from := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)
	to := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	_, err := svc.GetCampaignIncidentTimeline(context.Background(), GetCampaignIncidentTimelineInput{
		WorkspaceID: "ws-1",
		CampaignID:  "camp-1",
		UserID:      "user-1",
		From:        from,
		To:          to,
	})
	if err == nil {
		t.Fatal("expected time range error")
	}
}

func TestGetCampaignIncidentTimeline_RepoError(t *testing.T) {
	repoErr := errors.New("clickhouse down")
	svc := newTestServiceWithForensicQuery(
		nil, nil, nil,
		&stubForensicQueryRepo{
			GetCampaignIncidentTimelineFunc: func(_ context.Context, _, _ string, _, _ time.Time) (*domain.CampaignIncidentTimelineResult, error) {
				return nil, repoErr
			},
		},
		nil, nil, nil,
	)

	_, err := svc.GetCampaignIncidentTimeline(context.Background(), GetCampaignIncidentTimelineInput{
		WorkspaceID: "ws-1",
		CampaignID:  "camp-1",
		UserID:      "user-1",
	})
	if err != repoErr {
		t.Fatalf("expected repo error, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Stub Operations Query Repo
// ---------------------------------------------------------------------------

type stubOperationsQueryRepo struct {
	GetOutboxLagFunc                 func(ctx context.Context, workspaceID string, from, to time.Time, source string) (*domain.OutboxLagResult, error)
	GetConsumerFailuresFunc          func(ctx context.Context, workspaceID string, from, to time.Time, source string) (*domain.ConsumerFailureResult, error)
	GetDLQVolumeFunc                 func(ctx context.Context, workspaceID string, from, to time.Time, source string) (*domain.DLQResult, error)
	GetWebhookDeliveryTimeSeriesFunc func(ctx context.Context, workspaceID string, from, to time.Time, status, interval string) (*domain.WebhookDeliveryTimeSeriesResult, error)
	GetWebhookReliabilityFunc        func(ctx context.Context, workspaceID string, from, to time.Time, target string) (*domain.WebhookReliabilityResult, error)
}

func (s *stubOperationsQueryRepo) GetOutboxLag(ctx context.Context, workspaceID string, from, to time.Time, source string) (*domain.OutboxLagResult, error) {
	return s.GetOutboxLagFunc(ctx, workspaceID, from, to, source)
}
func (s *stubOperationsQueryRepo) GetConsumerFailures(ctx context.Context, workspaceID string, from, to time.Time, source string) (*domain.ConsumerFailureResult, error) {
	return s.GetConsumerFailuresFunc(ctx, workspaceID, from, to, source)
}
func (s *stubOperationsQueryRepo) GetDLQVolume(ctx context.Context, workspaceID string, from, to time.Time, source string) (*domain.DLQResult, error) {
	return s.GetDLQVolumeFunc(ctx, workspaceID, from, to, source)
}
func (s *stubOperationsQueryRepo) GetWebhookDeliveryTimeSeries(ctx context.Context, workspaceID string, from, to time.Time, status, interval string) (*domain.WebhookDeliveryTimeSeriesResult, error) {
	return s.GetWebhookDeliveryTimeSeriesFunc(ctx, workspaceID, from, to, status, interval)
}
func (s *stubOperationsQueryRepo) GetWebhookReliability(ctx context.Context, workspaceID string, from, to time.Time, target string) (*domain.WebhookReliabilityResult, error) {
	return s.GetWebhookReliabilityFunc(ctx, workspaceID, from, to, target)
}

func newTestServiceWithOperationsQuery(
	factRepo ports.EventFactRepository,
	projRead ports.ProjectionReadRepository,
	projWrite ports.ProjectionWriteRepository,
	operationsQuery ports.OperationsQueryRepository,
	txMgr ports.TransactionManager,
	outbox ports.OutboxWriter,
	checker ports.WorkspaceAccessChecker,
) *Service {
	s := newTestService(factRepo, projRead, projWrite, txMgr, outbox, checker)
	s.operationsQueryRepo = operationsQuery
	return s
}

// ---------------------------------------------------------------------------
// GetOutboxLag
// ---------------------------------------------------------------------------

func TestGetOutboxLag_Success(t *testing.T) {
	svc := newTestServiceWithOperationsQuery(
		nil, nil, nil,
		&stubOperationsQueryRepo{
			GetOutboxLagFunc: func(_ context.Context, workspaceID string, _, _ time.Time, _ string) (*domain.OutboxLagResult, error) {
				return &domain.OutboxLagResult{
					Status:      "ready",
					WorkspaceID: workspaceID,
					Rows: []domain.OutboxLagRow{
						{Source: "delivery", EventType: "send_email", LagSeconds: 2.5, Count: 100, BucketStart: fixedTime.Format(time.RFC3339)},
					},
				}, nil
			},
		},
		nil, nil, nil,
	)

	result, err := svc.GetOutboxLag(context.Background(), GetOutboxLagInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
		From:        fixedTime.Add(-24 * time.Hour),
		To:          fixedTime,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "ready" {
		t.Fatalf("expected status ready, got %s", result.Status)
	}
	if len(result.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(result.Rows))
	}
	if result.Rows[0].Source != "delivery" {
		t.Fatalf("expected source delivery, got %s", result.Rows[0].Source)
	}
}

func TestGetOutboxLag_AccessDenied(t *testing.T) {
	svc := newTestServiceWithOperationsQuery(
		nil, nil, nil, nil, nil, nil,
		&stubAccessChecker{
			RequirePermissionFunc: func(_ context.Context, _, _, _ string) error {
				return errors.New("permission denied")
			},
		},
	)

	_, err := svc.GetOutboxLag(context.Background(), GetOutboxLagInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
	})
	if err == nil {
		t.Fatal("expected access error")
	}
}

func TestGetOutboxLag_NilRepo(t *testing.T) {
	svc := newTestService(nil, nil, nil, nil, nil, nil)

	_, err := svc.GetOutboxLag(context.Background(), GetOutboxLagInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
	})
	if err != domain.ErrAnalyticsStoreUnavailable {
		t.Fatalf("expected ErrAnalyticsQueryInvalid, got %v", err)
	}
}

func TestGetOutboxLag_InvalidTimeRange(t *testing.T) {
	svc := newTestServiceWithOperationsQuery(nil, nil, nil, nil, nil, nil, nil)

	_, err := svc.GetOutboxLag(context.Background(), GetOutboxLagInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
		From:        fixedTime,
		To:          fixedTime.Add(-24 * time.Hour),
	})
	if err == nil {
		t.Fatal("expected validation error for invalid time range")
	}
}

func TestGetOutboxLag_RepoError(t *testing.T) {
	repoErr := errors.New("clickhouse down")
	svc := newTestServiceWithOperationsQuery(
		nil, nil, nil,
		&stubOperationsQueryRepo{
			GetOutboxLagFunc: func(_ context.Context, _ string, _, _ time.Time, _ string) (*domain.OutboxLagResult, error) {
				return nil, repoErr
			},
		},
		nil, nil, nil,
	)

	_, err := svc.GetOutboxLag(context.Background(), GetOutboxLagInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
	})
	if err != repoErr {
		t.Fatalf("expected repo error, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// GetConsumerFailures
// ---------------------------------------------------------------------------

func TestGetConsumerFailures_Success(t *testing.T) {
	svc := newTestServiceWithOperationsQuery(
		nil, nil, nil,
		&stubOperationsQueryRepo{
			GetConsumerFailuresFunc: func(_ context.Context, workspaceID string, _, _ time.Time, _ string) (*domain.ConsumerFailureResult, error) {
				return &domain.ConsumerFailureResult{
					Status:      "ready",
					WorkspaceID: workspaceID,
					Rows: []domain.ConsumerFailureRow{
						{Source: "delivery", Consumer: "analytics_events", ErrorType: "timeout", Count: 5, BucketStart: fixedTime.Format(time.RFC3339)},
					},
				}, nil
			},
		},
		nil, nil, nil,
	)

	result, err := svc.GetConsumerFailures(context.Background(), GetConsumerFailuresInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "ready" {
		t.Fatalf("expected status ready, got %s", result.Status)
	}
	if len(result.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(result.Rows))
	}
}

func TestGetConsumerFailures_AccessDenied(t *testing.T) {
	svc := newTestServiceWithOperationsQuery(
		nil, nil, nil, nil, nil, nil,
		&stubAccessChecker{
			RequirePermissionFunc: func(_ context.Context, _, _, _ string) error {
				return errors.New("permission denied")
			},
		},
	)

	_, err := svc.GetConsumerFailures(context.Background(), GetConsumerFailuresInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
	})
	if err == nil {
		t.Fatal("expected access error")
	}
}

func TestGetConsumerFailures_NilRepo(t *testing.T) {
	svc := newTestService(nil, nil, nil, nil, nil, nil)

	_, err := svc.GetConsumerFailures(context.Background(), GetConsumerFailuresInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
	})
	if err != domain.ErrAnalyticsStoreUnavailable {
		t.Fatalf("expected ErrAnalyticsQueryInvalid, got %v", err)
	}
}

func TestGetConsumerFailures_InvalidTimeRange(t *testing.T) {
	svc := newTestServiceWithOperationsQuery(nil, nil, nil, nil, nil, nil, nil)

	_, err := svc.GetConsumerFailures(context.Background(), GetConsumerFailuresInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
		From:        fixedTime,
		To:          fixedTime.Add(-24 * time.Hour),
	})
	if err == nil {
		t.Fatal("expected validation error for invalid time range")
	}
}

func TestGetConsumerFailures_RepoError(t *testing.T) {
	repoErr := errors.New("clickhouse down")
	svc := newTestServiceWithOperationsQuery(
		nil, nil, nil,
		&stubOperationsQueryRepo{
			GetConsumerFailuresFunc: func(_ context.Context, _ string, _, _ time.Time, _ string) (*domain.ConsumerFailureResult, error) {
				return nil, repoErr
			},
		},
		nil, nil, nil,
	)

	_, err := svc.GetConsumerFailures(context.Background(), GetConsumerFailuresInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
	})
	if err != repoErr {
		t.Fatalf("expected repo error, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// GetDLQVolume
// ---------------------------------------------------------------------------

func TestGetDLQVolume_Success(t *testing.T) {
	svc := newTestServiceWithOperationsQuery(
		nil, nil, nil,
		&stubOperationsQueryRepo{
			GetDLQVolumeFunc: func(_ context.Context, workspaceID string, _, _ time.Time, _ string) (*domain.DLQResult, error) {
				return &domain.DLQResult{
					Status:      "ready",
					WorkspaceID: workspaceID,
					Rows: []domain.DLQRow{
						{Source: "delivery", EventType: "send_email", Count: 10, BucketStart: fixedTime.Format(time.RFC3339)},
					},
				}, nil
			},
		},
		nil, nil, nil,
	)

	result, err := svc.GetDLQVolume(context.Background(), GetDLQVolumeInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "ready" {
		t.Fatalf("expected status ready, got %s", result.Status)
	}
	if len(result.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(result.Rows))
	}
}

func TestGetDLQVolume_AccessDenied(t *testing.T) {
	svc := newTestServiceWithOperationsQuery(
		nil, nil, nil, nil, nil, nil,
		&stubAccessChecker{
			RequirePermissionFunc: func(_ context.Context, _, _, _ string) error {
				return errors.New("permission denied")
			},
		},
	)

	_, err := svc.GetDLQVolume(context.Background(), GetDLQVolumeInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
	})
	if err == nil {
		t.Fatal("expected access error")
	}
}

func TestGetDLQVolume_NilRepo(t *testing.T) {
	svc := newTestService(nil, nil, nil, nil, nil, nil)

	_, err := svc.GetDLQVolume(context.Background(), GetDLQVolumeInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
	})
	if err != domain.ErrAnalyticsStoreUnavailable {
		t.Fatalf("expected ErrAnalyticsQueryInvalid, got %v", err)
	}
}

func TestGetDLQVolume_InvalidTimeRange(t *testing.T) {
	svc := newTestServiceWithOperationsQuery(nil, nil, nil, nil, nil, nil, nil)

	_, err := svc.GetDLQVolume(context.Background(), GetDLQVolumeInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
		From:        fixedTime,
		To:          fixedTime.Add(-24 * time.Hour),
	})
	if err == nil {
		t.Fatal("expected validation error for invalid time range")
	}
}

func TestGetDLQVolume_RepoError(t *testing.T) {
	repoErr := errors.New("clickhouse down")
	svc := newTestServiceWithOperationsQuery(
		nil, nil, nil,
		&stubOperationsQueryRepo{
			GetDLQVolumeFunc: func(_ context.Context, _ string, _, _ time.Time, _ string) (*domain.DLQResult, error) {
				return nil, repoErr
			},
		},
		nil, nil, nil,
	)

	_, err := svc.GetDLQVolume(context.Background(), GetDLQVolumeInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
	})
	if err != repoErr {
		t.Fatalf("expected repo error, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// GetWebhookDeliveryTimeSeries
// ---------------------------------------------------------------------------

func TestGetWebhookDeliveryTimeSeries_Success(t *testing.T) {
	svc := newTestServiceWithOperationsQuery(
		nil, nil, nil,
		&stubOperationsQueryRepo{
			GetWebhookDeliveryTimeSeriesFunc: func(_ context.Context, workspaceID string, _, _ time.Time, _, _ string) (*domain.WebhookDeliveryTimeSeriesResult, error) {
				return &domain.WebhookDeliveryTimeSeriesResult{
					Status:      "ready",
					WorkspaceID: workspaceID,
					Buckets: []domain.WebhookDeliveryTimeSeriesBucket{
						{BucketStart: fixedTime.Format(time.RFC3339), Status: "success", Count: 100},
					},
				}, nil
			},
		},
		nil, nil, nil,
	)

	result, err := svc.GetWebhookDeliveryTimeSeries(context.Background(), GetWebhookDeliveryTimeSeriesInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
		From:        fixedTime.Add(-24 * time.Hour),
		To:          fixedTime,
		Interval:    "day",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "ready" {
		t.Fatalf("expected status ready, got %s", result.Status)
	}
	if len(result.Buckets) != 1 {
		t.Fatalf("expected 1 bucket, got %d", len(result.Buckets))
	}
}

func TestGetWebhookDeliveryTimeSeries_AccessDenied(t *testing.T) {
	svc := newTestServiceWithOperationsQuery(
		nil, nil, nil, nil, nil, nil,
		&stubAccessChecker{
			RequirePermissionFunc: func(_ context.Context, _, _, _ string) error {
				return errors.New("permission denied")
			},
		},
	)

	_, err := svc.GetWebhookDeliveryTimeSeries(context.Background(), GetWebhookDeliveryTimeSeriesInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
	})
	if err == nil {
		t.Fatal("expected access error")
	}
}

func TestGetWebhookDeliveryTimeSeries_NilRepo(t *testing.T) {
	svc := newTestService(nil, nil, nil, nil, nil, nil)

	_, err := svc.GetWebhookDeliveryTimeSeries(context.Background(), GetWebhookDeliveryTimeSeriesInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
	})
	if err != domain.ErrAnalyticsStoreUnavailable {
		t.Fatalf("expected ErrAnalyticsQueryInvalid, got %v", err)
	}
}

func TestGetWebhookDeliveryTimeSeries_InvalidInterval(t *testing.T) {
	svc := newTestServiceWithOperationsQuery(nil, nil, nil, nil, nil, nil, nil)

	_, err := svc.GetWebhookDeliveryTimeSeries(context.Background(), GetWebhookDeliveryTimeSeriesInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
		Interval:    "invalid",
	})
	if err == nil {
		t.Fatal("expected validation error for invalid interval")
	}
}

func TestGetWebhookDeliveryTimeSeries_InvalidTimeRange(t *testing.T) {
	svc := newTestServiceWithOperationsQuery(nil, nil, nil, nil, nil, nil, nil)

	_, err := svc.GetWebhookDeliveryTimeSeries(context.Background(), GetWebhookDeliveryTimeSeriesInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
		From:        fixedTime,
		To:          fixedTime.Add(-24 * time.Hour),
	})
	if err == nil {
		t.Fatal("expected validation error for invalid time range")
	}
}

func TestGetWebhookDeliveryTimeSeries_RepoError(t *testing.T) {
	repoErr := errors.New("clickhouse down")
	svc := newTestServiceWithOperationsQuery(
		nil, nil, nil,
		&stubOperationsQueryRepo{
			GetWebhookDeliveryTimeSeriesFunc: func(_ context.Context, _ string, _, _ time.Time, _, _ string) (*domain.WebhookDeliveryTimeSeriesResult, error) {
				return nil, repoErr
			},
		},
		nil, nil, nil,
	)

	_, err := svc.GetWebhookDeliveryTimeSeries(context.Background(), GetWebhookDeliveryTimeSeriesInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
	})
	if err != repoErr {
		t.Fatalf("expected repo error, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// GetWebhookReliability
// ---------------------------------------------------------------------------

func TestGetWebhookReliability_Success(t *testing.T) {
	svc := newTestServiceWithOperationsQuery(
		nil, nil, nil,
		&stubOperationsQueryRepo{
			GetWebhookReliabilityFunc: func(_ context.Context, workspaceID string, _, _ time.Time, _ string) (*domain.WebhookReliabilityResult, error) {
				return &domain.WebhookReliabilityResult{
					Status:      "ready",
					WorkspaceID: workspaceID,
					Rows: []domain.WebhookReliabilityRow{
						{Source: "webhooks", Target: "https://example.com/hook", TotalCount: 100, Succeeded: 95, Failed: 3, Retried: 2, SuccessRate: 95},
					},
				}, nil
			},
		},
		nil, nil, nil,
	)

	result, err := svc.GetWebhookReliability(context.Background(), GetWebhookReliabilityInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "ready" {
		t.Fatalf("expected status ready, got %s", result.Status)
	}
	if len(result.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(result.Rows))
	}
}

func TestGetWebhookReliability_AccessDenied(t *testing.T) {
	svc := newTestServiceWithOperationsQuery(
		nil, nil, nil, nil, nil, nil,
		&stubAccessChecker{
			RequirePermissionFunc: func(_ context.Context, _, _, _ string) error {
				return errors.New("permission denied")
			},
		},
	)

	_, err := svc.GetWebhookReliability(context.Background(), GetWebhookReliabilityInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
	})
	if err == nil {
		t.Fatal("expected access error")
	}
}

func TestGetWebhookReliability_NilRepo(t *testing.T) {
	svc := newTestService(nil, nil, nil, nil, nil, nil)

	_, err := svc.GetWebhookReliability(context.Background(), GetWebhookReliabilityInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
	})
	if err != domain.ErrAnalyticsStoreUnavailable {
		t.Fatalf("expected ErrAnalyticsQueryInvalid, got %v", err)
	}
}

func TestGetWebhookReliability_InvalidTimeRange(t *testing.T) {
	svc := newTestServiceWithOperationsQuery(nil, nil, nil, nil, nil, nil, nil)

	_, err := svc.GetWebhookReliability(context.Background(), GetWebhookReliabilityInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
		From:        fixedTime,
		To:          fixedTime.Add(-24 * time.Hour),
	})
	if err == nil {
		t.Fatal("expected validation error for invalid time range")
	}
}

func TestGetWebhookReliability_RepoError(t *testing.T) {
	repoErr := errors.New("clickhouse down")
	svc := newTestServiceWithOperationsQuery(
		nil, nil, nil,
		&stubOperationsQueryRepo{
			GetWebhookReliabilityFunc: func(_ context.Context, _ string, _, _ time.Time, _ string) (*domain.WebhookReliabilityResult, error) {
				return nil, repoErr
			},
		},
		nil, nil, nil,
	)

	_, err := svc.GetWebhookReliability(context.Background(), GetWebhookReliabilityInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
	})
	if err != repoErr {
		t.Fatalf("expected repo error, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Phase 6: Product Usage, Risk, Forecasting, and Anomalies
// ---------------------------------------------------------------------------

type stubUsageQueryRepo struct {
	GetUsageTimeSeriesFunc     func(ctx context.Context, workspaceID string, from, to time.Time, interval string) (*domain.UsageTimeSeriesResult, error)
	GetUsageFeaturesFunc       func(ctx context.Context, workspaceID string, from, to time.Time) (*domain.UsageFeaturesResult, error)
	GetRiskSignalsFunc         func(ctx context.Context, workspaceID string, from, to time.Time) (*domain.RiskSignalsResult, error)
	GetSendVolumeForecastFunc  func(ctx context.Context, workspaceID string, from, to time.Time) (*domain.SendVolumeForecastResult, error)
	GetAnomaliesFunc           func(ctx context.Context, workspaceID string, from, to time.Time) (*domain.AnomaliesResult, error)
	ListDistinctWorkspacesFunc func(ctx context.Context, since time.Time) ([]string, error)
}

func (s *stubUsageQueryRepo) GetUsageTimeSeries(ctx context.Context, workspaceID string, from, to time.Time, interval string) (*domain.UsageTimeSeriesResult, error) {
	return s.GetUsageTimeSeriesFunc(ctx, workspaceID, from, to, interval)
}
func (s *stubUsageQueryRepo) GetUsageFeatures(ctx context.Context, workspaceID string, from, to time.Time) (*domain.UsageFeaturesResult, error) {
	return s.GetUsageFeaturesFunc(ctx, workspaceID, from, to)
}
func (s *stubUsageQueryRepo) GetRiskSignals(ctx context.Context, workspaceID string, from, to time.Time) (*domain.RiskSignalsResult, error) {
	return s.GetRiskSignalsFunc(ctx, workspaceID, from, to)
}
func (s *stubUsageQueryRepo) GetSendVolumeForecast(ctx context.Context, workspaceID string, from, to time.Time) (*domain.SendVolumeForecastResult, error) {
	return s.GetSendVolumeForecastFunc(ctx, workspaceID, from, to)
}
func (s *stubUsageQueryRepo) GetAnomalies(ctx context.Context, workspaceID string, from, to time.Time) (*domain.AnomaliesResult, error) {
	return s.GetAnomaliesFunc(ctx, workspaceID, from, to)
}
func (s *stubUsageQueryRepo) ListDistinctWorkspaces(ctx context.Context, since time.Time) ([]string, error) {
	return s.ListDistinctWorkspacesFunc(ctx, since)
}

func newTestServiceWithUsageQuery(
	factRepo ports.EventFactRepository,
	projRead ports.ProjectionReadRepository,
	projWrite ports.ProjectionWriteRepository,
	usageQuery ports.UsageQueryRepository,
	txMgr ports.TransactionManager,
	outbox ports.OutboxWriter,
	checker ports.WorkspaceAccessChecker,
) *Service {
	s := newTestService(factRepo, projRead, projWrite, txMgr, outbox, checker)
	s.usageQueryRepo = usageQuery
	return s
}

// ── GetUsageTimeSeries ──

func TestGetUsageTimeSeries_Success(t *testing.T) {
	svc := newTestServiceWithUsageQuery(
		nil, nil, nil,
		&stubUsageQueryRepo{
			GetUsageTimeSeriesFunc: func(_ context.Context, workspaceID string, _, _ time.Time, interval string) (*domain.UsageTimeSeriesResult, error) {
				return &domain.UsageTimeSeriesResult{
					Status:      "ready",
					WorkspaceID: workspaceID,
					Buckets: []domain.UsageTimeSeriesBucket{
						{BucketStart: fixedTime, EventType: "delivered", Count: 100},
					},
				}, nil
			},
		},
		nil, nil, nil,
	)

	result, err := svc.GetUsageTimeSeries(context.Background(), GetUsageTimeSeriesInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
		From:        fixedTime.Add(-24 * time.Hour),
		To:          fixedTime,
		Interval:    "day",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "ready" {
		t.Fatalf("expected status ready, got %s", result.Status)
	}
	if len(result.Buckets) != 1 {
		t.Fatalf("expected 1 bucket, got %d", len(result.Buckets))
	}
}

func TestGetUsageTimeSeries_AccessDenied(t *testing.T) {
	svc := newTestServiceWithUsageQuery(
		nil, nil, nil, nil, nil, nil,
		&stubAccessChecker{
			RequirePermissionFunc: func(_ context.Context, _, _, _ string) error {
				return errors.New("permission denied")
			},
		},
	)

	_, err := svc.GetUsageTimeSeries(context.Background(), GetUsageTimeSeriesInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
	})
	if err == nil {
		t.Fatal("expected access error")
	}
}

func TestGetUsageTimeSeries_NilRepo(t *testing.T) {
	svc := newTestService(nil, nil, nil, nil, nil, nil)

	_, err := svc.GetUsageTimeSeries(context.Background(), GetUsageTimeSeriesInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
	})
	if err != domain.ErrAnalyticsStoreUnavailable {
		t.Fatalf("expected ErrAnalyticsQueryInvalid, got %v", err)
	}
}

func TestGetUsageTimeSeries_InvalidInterval(t *testing.T) {
	svc := newTestServiceWithUsageQuery(nil, nil, nil, nil, nil, nil, nil)

	_, err := svc.GetUsageTimeSeries(context.Background(), GetUsageTimeSeriesInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
		Interval:    "invalid",
	})
	if err == nil {
		t.Fatal("expected validation error for invalid interval")
	}
}

func TestGetUsageTimeSeries_InvalidTimeRange(t *testing.T) {
	svc := newTestServiceWithUsageQuery(nil, nil, nil, nil, nil, nil, nil)

	_, err := svc.GetUsageTimeSeries(context.Background(), GetUsageTimeSeriesInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
		From:        fixedTime,
		To:          fixedTime.Add(-24 * time.Hour),
	})
	if err == nil {
		t.Fatal("expected validation error for invalid time range")
	}
}

func TestGetUsageTimeSeries_RepoError(t *testing.T) {
	repoErr := errors.New("clickhouse down")
	svc := newTestServiceWithUsageQuery(
		nil, nil, nil,
		&stubUsageQueryRepo{
			GetUsageTimeSeriesFunc: func(_ context.Context, _ string, _, _ time.Time, _ string) (*domain.UsageTimeSeriesResult, error) {
				return nil, repoErr
			},
		},
		nil, nil, nil,
	)

	_, err := svc.GetUsageTimeSeries(context.Background(), GetUsageTimeSeriesInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
	})
	if err != repoErr {
		t.Fatalf("expected repo error, got %v", err)
	}
}

// ── GetUsageFeatures ──

func TestGetUsageFeatures_Success(t *testing.T) {
	svc := newTestServiceWithUsageQuery(
		nil, nil, nil,
		&stubUsageQueryRepo{
			GetUsageFeaturesFunc: func(_ context.Context, workspaceID string, _, _ time.Time) (*domain.UsageFeaturesResult, error) {
				return &domain.UsageFeaturesResult{
					Status:      "ready",
					WorkspaceID: workspaceID,
					Rows: []domain.UsageFeatureRow{
						{Feature: "campaigns", ActiveCount: 5, EventCount: 100, BucketStart: fixedTime.Format(time.RFC3339)},
					},
				}, nil
			},
		},
		nil, nil, nil,
	)

	result, err := svc.GetUsageFeatures(context.Background(), GetUsageFeaturesInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "ready" {
		t.Fatalf("expected status ready, got %s", result.Status)
	}
	if len(result.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(result.Rows))
	}
}

func TestGetUsageFeatures_AccessDenied(t *testing.T) {
	svc := newTestServiceWithUsageQuery(
		nil, nil, nil, nil, nil, nil,
		&stubAccessChecker{
			RequirePermissionFunc: func(_ context.Context, _, _, _ string) error {
				return errors.New("permission denied")
			},
		},
	)

	_, err := svc.GetUsageFeatures(context.Background(), GetUsageFeaturesInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
	})
	if err == nil {
		t.Fatal("expected access error")
	}
}

func TestGetUsageFeatures_NilRepo(t *testing.T) {
	svc := newTestService(nil, nil, nil, nil, nil, nil)

	_, err := svc.GetUsageFeatures(context.Background(), GetUsageFeaturesInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
	})
	if err != domain.ErrAnalyticsStoreUnavailable {
		t.Fatalf("expected ErrAnalyticsQueryInvalid, got %v", err)
	}
}

func TestGetUsageFeatures_InvalidTimeRange(t *testing.T) {
	svc := newTestServiceWithUsageQuery(nil, nil, nil, nil, nil, nil, nil)

	_, err := svc.GetUsageFeatures(context.Background(), GetUsageFeaturesInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
		From:        fixedTime,
		To:          fixedTime.Add(-24 * time.Hour),
	})
	if err == nil {
		t.Fatal("expected validation error for invalid time range")
	}
}

func TestGetUsageFeatures_RepoError(t *testing.T) {
	repoErr := errors.New("clickhouse down")
	svc := newTestServiceWithUsageQuery(
		nil, nil, nil,
		&stubUsageQueryRepo{
			GetUsageFeaturesFunc: func(_ context.Context, _ string, _, _ time.Time) (*domain.UsageFeaturesResult, error) {
				return nil, repoErr
			},
		},
		nil, nil, nil,
	)

	_, err := svc.GetUsageFeatures(context.Background(), GetUsageFeaturesInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
	})
	if err != repoErr {
		t.Fatalf("expected repo error, got %v", err)
	}
}

// ── GetRiskSignals ──

func TestGetRiskSignals_Success(t *testing.T) {
	svc := newTestServiceWithUsageQuery(
		nil, nil, nil,
		&stubUsageQueryRepo{
			GetRiskSignalsFunc: func(_ context.Context, workspaceID string, _, _ time.Time) (*domain.RiskSignalsResult, error) {
				return &domain.RiskSignalsResult{
					Status:      "ready",
					WorkspaceID: workspaceID,
					Signals: []domain.RiskSignalRow{
						{SignalType: "high_bounce_rate", Severity: "high", Metric: "bounce_rate", Value: 15.0, Threshold: 5.0, DetectedAt: fixedTime.Format(time.RFC3339), CampaignID: "camp-1"},
					},
				}, nil
			},
		},
		nil, nil, nil,
	)

	result, err := svc.GetRiskSignals(context.Background(), GetRiskSignalsInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "ready" {
		t.Fatalf("expected status ready, got %s", result.Status)
	}
	if len(result.Signals) != 1 {
		t.Fatalf("expected 1 signal, got %d", len(result.Signals))
	}
}

func TestGetRiskSignals_AccessDenied(t *testing.T) {
	svc := newTestServiceWithUsageQuery(
		nil, nil, nil, nil, nil, nil,
		&stubAccessChecker{
			RequirePermissionFunc: func(_ context.Context, _, _, _ string) error {
				return errors.New("permission denied")
			},
		},
	)

	_, err := svc.GetRiskSignals(context.Background(), GetRiskSignalsInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
	})
	if err == nil {
		t.Fatal("expected access error")
	}
}

func TestGetRiskSignals_NilRepo(t *testing.T) {
	svc := newTestService(nil, nil, nil, nil, nil, nil)

	_, err := svc.GetRiskSignals(context.Background(), GetRiskSignalsInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
	})
	if err != domain.ErrAnalyticsStoreUnavailable {
		t.Fatalf("expected ErrAnalyticsQueryInvalid, got %v", err)
	}
}

func TestGetRiskSignals_RepoError(t *testing.T) {
	repoErr := errors.New("clickhouse down")
	svc := newTestServiceWithUsageQuery(
		nil, nil, nil,
		&stubUsageQueryRepo{
			GetRiskSignalsFunc: func(_ context.Context, _ string, _, _ time.Time) (*domain.RiskSignalsResult, error) {
				return nil, repoErr
			},
		},
		nil, nil, nil,
	)

	_, err := svc.GetRiskSignals(context.Background(), GetRiskSignalsInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
	})
	if err != repoErr {
		t.Fatalf("expected repo error, got %v", err)
	}
}

// ── GetSendVolumeForecast ──

func TestGetSendVolumeForecast_Success(t *testing.T) {
	svc := newTestServiceWithUsageQuery(
		nil, nil, nil,
		&stubUsageQueryRepo{
			GetSendVolumeForecastFunc: func(_ context.Context, workspaceID string, _, _ time.Time) (*domain.SendVolumeForecastResult, error) {
				return &domain.SendVolumeForecastResult{
					Status:      "ready",
					WorkspaceID: workspaceID,
					Rows: []domain.SendVolumeForecastRow{
						{BucketStart: fixedTime.Format(time.RFC3339), ForecastLow: 80, ForecastMid: 100, ForecastHigh: 120, Confidence: 0.95},
					},
				}, nil
			},
		},
		nil, nil, nil,
	)

	result, err := svc.GetSendVolumeForecast(context.Background(), GetSendVolumeForecastInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "ready" {
		t.Fatalf("expected status ready, got %s", result.Status)
	}
	if len(result.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(result.Rows))
	}
}

func TestGetSendVolumeForecast_AccessDenied(t *testing.T) {
	svc := newTestServiceWithUsageQuery(
		nil, nil, nil, nil, nil, nil,
		&stubAccessChecker{
			RequirePermissionFunc: func(_ context.Context, _, _, _ string) error {
				return errors.New("permission denied")
			},
		},
	)

	_, err := svc.GetSendVolumeForecast(context.Background(), GetSendVolumeForecastInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
	})
	if err == nil {
		t.Fatal("expected access error")
	}
}

func TestGetSendVolumeForecast_NilRepo(t *testing.T) {
	svc := newTestService(nil, nil, nil, nil, nil, nil)

	_, err := svc.GetSendVolumeForecast(context.Background(), GetSendVolumeForecastInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
	})
	if err != domain.ErrAnalyticsStoreUnavailable {
		t.Fatalf("expected ErrAnalyticsQueryInvalid, got %v", err)
	}
}

func TestGetSendVolumeForecast_RepoError(t *testing.T) {
	repoErr := errors.New("clickhouse down")
	svc := newTestServiceWithUsageQuery(
		nil, nil, nil,
		&stubUsageQueryRepo{
			GetSendVolumeForecastFunc: func(_ context.Context, _ string, _, _ time.Time) (*domain.SendVolumeForecastResult, error) {
				return nil, repoErr
			},
		},
		nil, nil, nil,
	)

	_, err := svc.GetSendVolumeForecast(context.Background(), GetSendVolumeForecastInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
	})
	if err != repoErr {
		t.Fatalf("expected repo error, got %v", err)
	}
}

// ── GetAnomalies ──

func TestGetAnomalies_Success(t *testing.T) {
	svc := newTestServiceWithUsageQuery(
		nil, nil, nil,
		&stubUsageQueryRepo{
			GetAnomaliesFunc: func(_ context.Context, workspaceID string, _, _ time.Time) (*domain.AnomaliesResult, error) {
				return &domain.AnomaliesResult{
					Status:      "ready",
					WorkspaceID: workspaceID,
					Anomalies: []domain.AnomalyRow{
						{AnomalyID: "anomaly_1", AnomalyType: "volume_spike", Severity: "high", Metric: "bounced_count", Observed: 50, Expected: 10, Deviation: 4.5, DetectedAt: fixedTime.Format(time.RFC3339), WindowStart: fixedTime.Format(time.RFC3339), WindowEnd: fixedTime.Format(time.RFC3339)},
					},
				}, nil
			},
		},
		nil, nil, nil,
	)

	result, err := svc.GetAnomalies(context.Background(), GetAnomaliesInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "ready" {
		t.Fatalf("expected status ready, got %s", result.Status)
	}
	if len(result.Anomalies) != 1 {
		t.Fatalf("expected 1 anomaly, got %d", len(result.Anomalies))
	}
}

func TestGetAnomalies_AccessDenied(t *testing.T) {
	svc := newTestServiceWithUsageQuery(
		nil, nil, nil, nil, nil, nil,
		&stubAccessChecker{
			RequirePermissionFunc: func(_ context.Context, _, _, _ string) error {
				return errors.New("permission denied")
			},
		},
	)

	_, err := svc.GetAnomalies(context.Background(), GetAnomaliesInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
	})
	if err == nil {
		t.Fatal("expected access error")
	}
}

func TestGetAnomalies_NilRepo(t *testing.T) {
	svc := newTestService(nil, nil, nil, nil, nil, nil)

	_, err := svc.GetAnomalies(context.Background(), GetAnomaliesInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
	})
	if err != domain.ErrAnalyticsStoreUnavailable {
		t.Fatalf("expected ErrAnalyticsQueryInvalid, got %v", err)
	}
}

func TestGetAnomalies_RepoError(t *testing.T) {
	repoErr := errors.New("clickhouse down")
	svc := newTestServiceWithUsageQuery(
		nil, nil, nil,
		&stubUsageQueryRepo{
			GetAnomaliesFunc: func(_ context.Context, _ string, _, _ time.Time) (*domain.AnomaliesResult, error) {
				return nil, repoErr
			},
		},
		nil, nil, nil,
	)

	_, err := svc.GetAnomalies(context.Background(), GetAnomaliesInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
	})
	if err != repoErr {
		t.Fatalf("expected repo error, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// SyncFactsToClickHouse
// ---------------------------------------------------------------------------

type stubFactBatchRepo struct {
	ListFactsAfterCursorFunc func(ctx context.Context, cursorCreatedAt *time.Time, cursorID string, limit int) ([]domain.EmailEventFact, error)
}

func (s *stubFactBatchRepo) ListFactsAfterCursor(ctx context.Context, cursorCreatedAt *time.Time, cursorID string, limit int) ([]domain.EmailEventFact, error) {
	return s.ListFactsAfterCursorFunc(ctx, cursorCreatedAt, cursorID, limit)
}

type stubSyncStateRepo struct {
	GetSyncCursorFunc    func(ctx context.Context, streamName string) (*ports.SyncCursor, error)
	UpdateSyncCursorFunc func(ctx context.Context, cursor *ports.SyncCursor) error
}

func (s *stubSyncStateRepo) GetSyncCursor(ctx context.Context, streamName string) (*ports.SyncCursor, error) {
	return s.GetSyncCursorFunc(ctx, streamName)
}
func (s *stubSyncStateRepo) UpdateSyncCursor(ctx context.Context, cursor *ports.SyncCursor) error {
	return s.UpdateSyncCursorFunc(ctx, cursor)
}

type stubCHBatchWriter struct {
	CreateBatchFunc func(ctx context.Context, facts []domain.EmailEventFact) error
}

func (s *stubCHBatchWriter) CreateBatch(ctx context.Context, facts []domain.EmailEventFact) error {
	return s.CreateBatchFunc(ctx, facts)
}

func newTestServiceWithSyncRepos(
	factBatchRepo ports.FactBatchRepository,
	syncStateRepo ports.SyncStateRepository,
	chBatchWriter ports.ClickHouseBatchWriter,
	clock func() time.Time,
) *Service {
	return NewService(Options{
		FactBatchRepo:         factBatchRepo,
		SyncStateRepo:         syncStateRepo,
		ClickHouseBatchWriter: chBatchWriter,
		Clock:                 clock,
		Logger:                slog.Default(),
	})
}

func TestSyncFactsToClickHouse_NoRows(t *testing.T) {
	svc := newTestServiceWithSyncRepos(
		&stubFactBatchRepo{
			ListFactsAfterCursorFunc: func(_ context.Context, _ *time.Time, _ string, _ int) ([]domain.EmailEventFact, error) {
				return nil, nil
			},
		},
		&stubSyncStateRepo{
			GetSyncCursorFunc: func(_ context.Context, _ string) (*ports.SyncCursor, error) {
				return &ports.SyncCursor{StreamName: "test"}, nil
			},
		},
		&stubCHBatchWriter{},
		func() time.Time { return fixedTime },
	)

	result, err := svc.SyncFactsToClickHouse(context.Background(), SyncFactsToClickHouseInput{
		StreamName: "test",
		BatchSize:  100,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.SyncedCount != 0 {
		t.Fatalf("expected 0 synced, got %d", result.SyncedCount)
	}
}

func TestSyncFactsToClickHouse_OneBatch(t *testing.T) {
	now := time.Date(2025, 6, 13, 12, 0, 0, 0, time.UTC)
	facts := []domain.EmailEventFact{
		{ID: "fact-1", SourceEventID: "src-1", WorkspaceID: "ws-1", EventType: domain.EventTypeDelivered, OccurredAt: now, ReceivedAt: now, CreatedAt: now},
		{ID: "fact-2", SourceEventID: "src-2", WorkspaceID: "ws-1", EventType: domain.EventTypeDelivered, OccurredAt: now, ReceivedAt: now, CreatedAt: now.Add(1 * time.Second)},
	}

	var capturedFacts []domain.EmailEventFact
	var updatedCursor *ports.SyncCursor

	svc := newTestServiceWithSyncRepos(
		&stubFactBatchRepo{
			ListFactsAfterCursorFunc: func(_ context.Context, _ *time.Time, _ string, _ int) ([]domain.EmailEventFact, error) {
				return facts, nil
			},
		},
		&stubSyncStateRepo{
			GetSyncCursorFunc: func(_ context.Context, _ string) (*ports.SyncCursor, error) {
				return &ports.SyncCursor{StreamName: "test"}, nil
			},
			UpdateSyncCursorFunc: func(_ context.Context, cursor *ports.SyncCursor) error {
				updatedCursor = cursor
				return nil
			},
		},
		&stubCHBatchWriter{
			CreateBatchFunc: func(_ context.Context, f []domain.EmailEventFact) error {
				capturedFacts = f
				return nil
			},
		},
		func() time.Time { return now },
	)

	result, err := svc.SyncFactsToClickHouse(context.Background(), SyncFactsToClickHouseInput{
		StreamName: "test",
		BatchSize:  100,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.SyncedCount != 2 {
		t.Fatalf("expected 2 synced, got %d", result.SyncedCount)
	}
	if result.LastFactID != "fact-2" {
		t.Fatalf("expected fact-2, got %s", result.LastFactID)
	}
	if len(capturedFacts) != 2 {
		t.Fatalf("expected 2 facts, got %d", len(capturedFacts))
	}
	if updatedCursor == nil {
		t.Fatal("expected cursor to be updated")
	}
	if updatedCursor.LastFactID != "fact-2" {
		t.Fatalf("expected cursor last_fact_id fact-2, got %s", updatedCursor.LastFactID)
	}
}

func TestSyncFactsToClickHouse_NilRepos(t *testing.T) {
	svc := NewService(Options{Logger: slog.Default()})
	_, err := svc.SyncFactsToClickHouse(context.Background(), SyncFactsToClickHouseInput{
		StreamName: "test",
	})
	if err != domain.ErrAnalyticsStoreUnavailable {
		t.Fatalf("expected ErrAnalyticsQueryInvalid, got %v", err)
	}
}

func TestSyncFactsToClickHouse_CHFailureDoesNotAdvanceCursor(t *testing.T) {
	chErr := errors.New("clickhouse connection refused")
	svc := newTestServiceWithSyncRepos(
		&stubFactBatchRepo{
			ListFactsAfterCursorFunc: func(_ context.Context, _ *time.Time, _ string, _ int) ([]domain.EmailEventFact, error) {
				return []domain.EmailEventFact{
					{ID: "fact-1", SourceEventID: "src-1", WorkspaceID: "ws-1", EventType: domain.EventTypeDelivered, CreatedAt: time.Now()},
				}, nil
			},
		},
		&stubSyncStateRepo{
			GetSyncCursorFunc: func(_ context.Context, _ string) (*ports.SyncCursor, error) {
				return &ports.SyncCursor{StreamName: "test"}, nil
			},
		},
		&stubCHBatchWriter{
			CreateBatchFunc: func(_ context.Context, _ []domain.EmailEventFact) error {
				return chErr
			},
		},
		time.Now,
	)

	_, err := svc.SyncFactsToClickHouse(context.Background(), SyncFactsToClickHouseInput{
		StreamName: "test",
		BatchSize:  100,
	})
	if err == nil {
		t.Fatal("expected error from ClickHouse failure")
	}
}

func TestSyncFactsToClickHouse_PGReadFailureDoesNotAdvanceCursor(t *testing.T) {
	pgErr := errors.New("postgres connection lost")
	svc := newTestServiceWithSyncRepos(
		&stubFactBatchRepo{
			ListFactsAfterCursorFunc: func(_ context.Context, _ *time.Time, _ string, _ int) ([]domain.EmailEventFact, error) {
				return nil, pgErr
			},
		},
		&stubSyncStateRepo{
			GetSyncCursorFunc: func(_ context.Context, _ string) (*ports.SyncCursor, error) {
				return &ports.SyncCursor{StreamName: "test"}, nil
			},
		},
		&stubCHBatchWriter{},
		time.Now,
	)

	_, err := svc.SyncFactsToClickHouse(context.Background(), SyncFactsToClickHouseInput{
		StreamName: "test",
		BatchSize:  100,
	})
	if err != pgErr {
		t.Fatalf("expected pg error, got %v", err)
	}
}
