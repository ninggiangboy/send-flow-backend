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
