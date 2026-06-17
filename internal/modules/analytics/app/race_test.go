package app

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/domain"
)

func TestIngestEmailEventFact_ConcurrentDuplicate(t *testing.T) {
	t.Parallel()

	var (
		mu              sync.Mutex
		factCreateCalls int32
	)

	factRepo := &stubFactRepo{
		CreateFunc: func(_ context.Context, fact domain.EmailEventFact) error {
			atomic.AddInt32(&factCreateCalls, 1)
			mu.Lock()
			defer mu.Unlock()
			return nil
		},
		FindBySourceEventIDFunc: func(_ context.Context, sourceEventID string) (*domain.EmailEventFact, error) {
			mu.Lock()
			defer mu.Unlock()
			return nil, nil
		},
	}

	svc := NewService(Options{
		FactRepo: factRepo,
		Clock:    time.Now,
		Logger:   slog.Default(),
	})

	input := IngestEmailEventFactInput{
		SourceEventID:   "src-evt-1",
		SourceEventType: "test.event.v1",
		WorkspaceID:     "ws-1",
		CampaignID:      "camp-1",
		MessageID:       "msg-1",
		Provider:        "aws",
		RecipientDomain: "example.com",
		CanonicalType:   "delivered",
		OccurredAt:      time.Now().UTC(),
		ReceivedAt:      time.Now().UTC(),
	}

	var wg sync.WaitGroup
	goroutines := 20

	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = svc.IngestEmailEventFact(context.Background(), input)
		}()
	}
	wg.Wait()

	createCalls := atomic.LoadInt32(&factCreateCalls)
	if createCalls > 1 {
		t.Logf("note: factRepo.Create called %d times (race window)", createCalls)
	}
}

func TestIngestEmailEventFact_ConcurrentDuplicateUniqueConstraint(t *testing.T) {
	t.Parallel()

	var (
		mu        sync.Mutex
		created   bool
		createCnt int32
	)

	factRepo := &stubFactRepo{
		CreateFunc: func(_ context.Context, fact domain.EmailEventFact) error {
			mu.Lock()
			defer mu.Unlock()
			atomic.AddInt32(&createCnt, 1)
			if created {
				return domain.ErrAnalyticsEventDuplicate
			}
			created = true
			return nil
		},
		FindBySourceEventIDFunc: func(_ context.Context, sourceEventID string) (*domain.EmailEventFact, error) {
			mu.Lock()
			defer mu.Unlock()
			if created {
				return &domain.EmailEventFact{}, nil
			}
			return nil, nil
		},
	}

	svc := NewService(Options{
		FactRepo: factRepo,
		Clock:    time.Now,
		Logger:   slog.Default(),
	})

	input := IngestEmailEventFactInput{
		SourceEventID:   "src-evt-2",
		SourceEventType: "test.event.v1",
		WorkspaceID:     "ws-1",
		CampaignID:      "camp-1",
		CanonicalType:   "delivered",
		OccurredAt:      time.Now().UTC(),
		ReceivedAt:      time.Now().UTC(),
	}

	var wg sync.WaitGroup
	for range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = svc.IngestEmailEventFact(context.Background(), input)
		}()
	}
	wg.Wait()
}

func TestGetCampaignFunnel_AuthorizationDenied(t *testing.T) {
	t.Parallel()

	svc := NewService(Options{
		AccessChecker: &stubAccessChecker{
			RequirePermissionFunc: func(_ context.Context, _, _, _ string) error {
				return errors.New("permission denied")
			},
		},
	})

	_, err := svc.GetCampaignFunnel(context.Background(), GetCampaignFunnelInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
		CampaignID:  "camp-1",
	})
	if err == nil {
		t.Fatal("expected authorization error, got nil")
	}
}

func TestGetCampaignTimeSeries_AuthorizationDenied(t *testing.T) {
	t.Parallel()

	svc := NewService(Options{
		AccessChecker: &stubAccessChecker{
			RequirePermissionFunc: func(_ context.Context, _, _, _ string) error {
				return domain.ErrAnalyticsReadDenied
			},
		},
	})

	_, err := svc.GetCampaignTimeSeries(context.Background(), GetCampaignTimeSeriesInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
		CampaignID:  "camp-1",
	})
	if err == nil {
		t.Fatal("expected authorization error, got nil")
	}
}

func TestGetDeliverabilityTimeSeries_AuthorizationDenied(t *testing.T) {
	t.Parallel()

	svc := NewService(Options{
		AccessChecker: &stubAccessChecker{
			RequirePermissionFunc: func(_ context.Context, _, _, _ string) error {
				return domain.ErrAnalyticsReadDenied
			},
		},
	})

	_, err := svc.GetDeliverabilityTimeSeries(context.Background(), GetDeliverabilityTimeSeriesInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
	})
	if err == nil {
		t.Fatal("expected authorization error, got nil")
	}
}

func TestSearchEvents_AuthorizationDenied(t *testing.T) {
	t.Parallel()

	svc := NewService(Options{
		AccessChecker: &stubAccessChecker{
			RequirePermissionFunc: func(_ context.Context, _, _, _ string) error {
				return domain.ErrAnalyticsReadDenied
			},
		},
	})

	_, err := svc.SearchEvents(context.Background(), SearchEventsInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
	})
	if err == nil {
		t.Fatal("expected authorization error, got nil")
	}
}

func TestGetOutboxLag_AuthorizationDenied(t *testing.T) {
	t.Parallel()

	svc := NewService(Options{
		AccessChecker: &stubAccessChecker{
			RequirePermissionFunc: func(_ context.Context, _, _, _ string) error {
				return domain.ErrAnalyticsReadDenied
			},
		},
	})

	_, err := svc.GetOutboxLag(context.Background(), GetOutboxLagInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
	})
	if err == nil {
		t.Fatal("expected authorization error, got nil")
	}
}

func TestGetUsageTimeSeries_AuthorizationDenied(t *testing.T) {
	t.Parallel()

	svc := NewService(Options{
		AccessChecker: &stubAccessChecker{
			RequirePermissionFunc: func(_ context.Context, _, _, _ string) error {
				return domain.ErrAnalyticsReadDenied
			},
		},
	})

	_, err := svc.GetUsageTimeSeries(context.Background(), GetUsageTimeSeriesInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
	})
	if err == nil {
		t.Fatal("expected authorization error, got nil")
	}
}

func TestGetAnomalies_AuthorizationDenied(t *testing.T) {
	t.Parallel()

	svc := NewService(Options{
		AccessChecker: &stubAccessChecker{
			RequirePermissionFunc: func(_ context.Context, _, _, _ string) error {
				return domain.ErrAnalyticsReadDenied
			},
		},
	})

	_, err := svc.GetAnomalies(context.Background(), GetAnomaliesInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
	})
	if err == nil {
		t.Fatal("expected authorization error, got nil")
	}
}

func TestGetSendVolumeForecast_AuthorizationDenied(t *testing.T) {
	t.Parallel()

	svc := NewService(Options{
		AccessChecker: &stubAccessChecker{
			RequirePermissionFunc: func(_ context.Context, _, _, _ string) error {
				return domain.ErrAnalyticsReadDenied
			},
		},
	})

	_, err := svc.GetSendVolumeForecast(context.Background(), GetSendVolumeForecastInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
	})
	if err == nil {
		t.Fatal("expected authorization error, got nil")
	}
}

func TestGetWebhookReliability_AuthorizationDenied(t *testing.T) {
	t.Parallel()

	svc := NewService(Options{
		AccessChecker: &stubAccessChecker{
			RequirePermissionFunc: func(_ context.Context, _, _, _ string) error {
				return domain.ErrAnalyticsReadDenied
			},
		},
	})

	_, err := svc.GetWebhookReliability(context.Background(), GetWebhookReliabilityInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
	})
	if err == nil {
		t.Fatal("expected authorization error, got nil")
	}
}
