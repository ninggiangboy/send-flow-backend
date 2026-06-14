package app

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/webhooks/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/webhooks/ports"
)

type mockConfigRead struct {
	findByIDFn        func(ctx context.Context, workspaceID, webhookID string) (*domain.WebhookConfig, error)
	listByWorkspaceFn func(ctx context.Context, workspaceID string) ([]domain.WebhookConfig, error)
	listSubscribedFn  func(ctx context.Context, workspaceID, eventType string) ([]domain.WebhookConfig, error)
}

func (m *mockConfigRead) FindByID(ctx context.Context, workspaceID, webhookID string) (*domain.WebhookConfig, error) {
	return m.findByIDFn(ctx, workspaceID, webhookID)
}
func (m *mockConfigRead) ListByWorkspace(ctx context.Context, workspaceID string) ([]domain.WebhookConfig, error) {
	return m.listByWorkspaceFn(ctx, workspaceID)
}
func (m *mockConfigRead) ListSubscribed(ctx context.Context, workspaceID, eventType string) ([]domain.WebhookConfig, error) {
	return m.listSubscribedFn(ctx, workspaceID, eventType)
}

type mockConfigWrite struct {
	createFn  func(ctx context.Context, config domain.WebhookConfig) error
	updateFn  func(ctx context.Context, config domain.WebhookConfig) error
	disableFn func(ctx context.Context, workspaceID, webhookID string, disabledAt time.Time) error
}

func (m *mockConfigWrite) Create(ctx context.Context, config domain.WebhookConfig) error {
	return m.createFn(ctx, config)
}
func (m *mockConfigWrite) Update(ctx context.Context, config domain.WebhookConfig) error {
	return m.updateFn(ctx, config)
}
func (m *mockConfigWrite) Disable(ctx context.Context, workspaceID, webhookID string, disabledAt time.Time) error {
	return m.disableFn(ctx, workspaceID, webhookID, disabledAt)
}

type mockDeliveryRead struct {
	findByIDFn              func(ctx context.Context, workspaceID, deliveryID string) (*domain.WebhookDelivery, error)
	findByWebhookAndEventFn func(ctx context.Context, webhookID, sourceEventID string) (*domain.WebhookDelivery, error)
	listByWorkspaceFn       func(ctx context.Context, workspaceID string, filter ports.DeliveryFilter) ([]domain.WebhookDelivery, string, error)
}

func (m *mockDeliveryRead) FindByID(ctx context.Context, workspaceID, deliveryID string) (*domain.WebhookDelivery, error) {
	return m.findByIDFn(ctx, workspaceID, deliveryID)
}
func (m *mockDeliveryRead) FindByWebhookAndEvent(ctx context.Context, webhookID, sourceEventID string) (*domain.WebhookDelivery, error) {
	return m.findByWebhookAndEventFn(ctx, webhookID, sourceEventID)
}
func (m *mockDeliveryRead) ListByWorkspace(ctx context.Context, workspaceID string, filter ports.DeliveryFilter) ([]domain.WebhookDelivery, string, error) {
	return m.listByWorkspaceFn(ctx, workspaceID, filter)
}

type mockDeliveryWrite struct {
	createFn         func(ctx context.Context, delivery domain.WebhookDelivery) error
	markDeliveringFn func(ctx context.Context, deliveryID string, now time.Time) error
	markSucceededFn  func(ctx context.Context, deliveryID string, result domain.DeliveryResult) error
	markFailedFn     func(ctx context.Context, deliveryID string, result domain.DeliveryResult) error
	scheduleRetryFn  func(ctx context.Context, deliveryID string, nextAttemptAt time.Time) error
	claimPendingFn   func(ctx context.Context, limit int, now time.Time) ([]domain.WebhookDelivery, error)
}

func (m *mockDeliveryWrite) Create(ctx context.Context, delivery domain.WebhookDelivery) error {
	return m.createFn(ctx, delivery)
}
func (m *mockDeliveryWrite) MarkDelivering(ctx context.Context, deliveryID string, now time.Time) error {
	return m.markDeliveringFn(ctx, deliveryID, now)
}
func (m *mockDeliveryWrite) MarkSucceeded(ctx context.Context, deliveryID string, result domain.DeliveryResult) error {
	return m.markSucceededFn(ctx, deliveryID, result)
}
func (m *mockDeliveryWrite) MarkFailed(ctx context.Context, deliveryID string, result domain.DeliveryResult) error {
	return m.markFailedFn(ctx, deliveryID, result)
}
func (m *mockDeliveryWrite) ScheduleRetry(ctx context.Context, deliveryID string, nextAttemptAt time.Time) error {
	return m.scheduleRetryFn(ctx, deliveryID, nextAttemptAt)
}
func (m *mockDeliveryWrite) ClaimPendingDeliveries(ctx context.Context, limit int, now time.Time) ([]domain.WebhookDelivery, error) {
	return m.claimPendingFn(ctx, limit, now)
}

type mockAttemptRead struct {
	listByDeliveryFn func(ctx context.Context, deliveryID string) ([]domain.WebhookDeliveryAttempt, error)
}

func (m *mockAttemptRead) ListByDelivery(ctx context.Context, deliveryID string) ([]domain.WebhookDeliveryAttempt, error) {
	return m.listByDeliveryFn(ctx, deliveryID)
}

type mockAttemptWrite struct {
	createFn func(ctx context.Context, attempt domain.WebhookDeliveryAttempt) error
}

func (m *mockAttemptWrite) Create(ctx context.Context, attempt domain.WebhookDeliveryAttempt) error {
	return m.createFn(ctx, attempt)
}

type mockTxManager struct {
	runInTxFn func(ctx context.Context, fn func(context.Context) error) error
}

func (m *mockTxManager) WithinTx(ctx context.Context, fn func(context.Context) error) error {
	return m.runInTxFn(ctx, fn)
}

type mockDeliverer struct {
	deliverFn func(ctx context.Context, req ports.DeliveryHTTPRequest) (ports.DeliveryHTTPResponse, error)
}

func (m *mockDeliverer) Deliver(ctx context.Context, req ports.DeliveryHTTPRequest) (ports.DeliveryHTTPResponse, error) {
	return m.deliverFn(ctx, req)
}

type mockAccessChecker struct {
	requirePermissionFn func(ctx context.Context, workspaceID, userID, permission string) error
}

func (m *mockAccessChecker) RequirePermission(ctx context.Context, workspaceID, userID, permission string) error {
	return m.requirePermissionFn(ctx, workspaceID, userID, permission)
}

func fixedIDGen() (string, error) {
	return "test-id", nil
}

func fixedClock() time.Time {
	return time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
}

func TestCreateWebhookConfig_Valid(t *testing.T) {
	svc := NewService(Options{
		ConfigWrite: &mockConfigWrite{
			createFn: func(ctx context.Context, config domain.WebhookConfig) error {
				return nil
			},
		},
		AccessChecker: &mockAccessChecker{
			requirePermissionFn: func(ctx context.Context, workspaceID, userID, permission string) error {
				return nil
			},
		},
		TxManager: &mockTxManager{
			runInTxFn: func(ctx context.Context, fn func(context.Context) error) error {
				return fn(ctx)
			},
		},
		Deliverer: &mockDeliverer{
			deliverFn: func(ctx context.Context, req ports.DeliveryHTTPRequest) (ports.DeliveryHTTPResponse, error) {
				return ports.DeliveryHTTPResponse{}, nil
			},
		},
		DeliveryRead:  &mockDeliveryRead{},
		DeliveryWrite: &mockDeliveryWrite{},
		AttemptRead:   &mockAttemptRead{},
		AttemptWrite:  &mockAttemptWrite{},
		ConfigRead:    &mockConfigRead{},
		IDGen:         fixedIDGen,
		Clock:         fixedClock,
		Logger:        slog.Default(),
	})

	result, err := svc.CreateWebhookConfig(context.Background(), CreateConfigInput{
		WorkspaceID:   "ws-1",
		UserID:        "user-1",
		Name:          "Test Webhook",
		TargetURL:     "https://example.com/hooks",
		Subscriptions: []string{"delivery.message.delivered.v1"},
		Now:           fixedClock(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Name != "Test Webhook" {
		t.Errorf("expected name 'Test Webhook', got %q", result.Name)
	}
	if result.RawSecret == "" {
		t.Error("expected raw secret to be non-empty")
	}
}

func TestCreateWebhookConfig_PermissionDenied(t *testing.T) {
	svc := NewService(Options{
		AccessChecker: &mockAccessChecker{
			requirePermissionFn: func(ctx context.Context, workspaceID, userID, permission string) error {
				return domain.ErrManageDenied
			},
		},
		ConfigRead:    &mockConfigRead{},
		ConfigWrite:   &mockConfigWrite{},
		DeliveryRead:  &mockDeliveryRead{},
		DeliveryWrite: &mockDeliveryWrite{},
		AttemptRead:   &mockAttemptRead{},
		AttemptWrite:  &mockAttemptWrite{},
		Deliverer: &mockDeliverer{
			deliverFn: func(ctx context.Context, req ports.DeliveryHTTPRequest) (ports.DeliveryHTTPResponse, error) {
				return ports.DeliveryHTTPResponse{}, nil
			},
		},
		IDGen:  fixedIDGen,
		Clock:  fixedClock,
		Logger: slog.Default(),
	})

	_, err := svc.CreateWebhookConfig(context.Background(), CreateConfigInput{
		WorkspaceID:   "ws-1",
		UserID:        "user-1",
		Name:          "Test Webhook",
		TargetURL:     "https://example.com/hooks",
		Subscriptions: []string{"delivery.message.delivered.v1"},
		Now:           fixedClock(),
	})
	if !errors.Is(err, domain.ErrManageDenied) {
		t.Errorf("expected ErrManageDenied, got %v", err)
	}
}

func TestCreateWebhookConfig_InvalidTargetURL(t *testing.T) {
	svc := NewService(Options{
		AccessChecker: &mockAccessChecker{
			requirePermissionFn: func(ctx context.Context, workspaceID, userID, permission string) error {
				return nil
			},
		},
		ConfigRead:    &mockConfigRead{},
		ConfigWrite:   &mockConfigWrite{},
		DeliveryRead:  &mockDeliveryRead{},
		DeliveryWrite: &mockDeliveryWrite{},
		AttemptRead:   &mockAttemptRead{},
		AttemptWrite:  &mockAttemptWrite{},
		Deliverer: &mockDeliverer{
			deliverFn: func(ctx context.Context, req ports.DeliveryHTTPRequest) (ports.DeliveryHTTPResponse, error) {
				return ports.DeliveryHTTPResponse{}, nil
			},
		},
		IDGen:  fixedIDGen,
		Clock:  fixedClock,
		Logger: slog.Default(),
	})

	_, err := svc.CreateWebhookConfig(context.Background(), CreateConfigInput{
		WorkspaceID:   "ws-1",
		UserID:        "user-1",
		Name:          "Test Webhook",
		TargetURL:     "ftp://invalid-scheme.com/hooks",
		Subscriptions: []string{"delivery.message.delivered.v1"},
		Now:           fixedClock(),
	})
	if !errors.Is(err, domain.ErrTargetURLInvalid) {
		t.Errorf("expected ErrTargetURLInvalid, got %v", err)
	}
}

func TestDisableWebhookConfig(t *testing.T) {
	svc := NewService(Options{
		ConfigRead: &mockConfigRead{
			findByIDFn: func(ctx context.Context, workspaceID, webhookID string) (*domain.WebhookConfig, error) {
				return &domain.WebhookConfig{
					ID:          webhookID,
					WorkspaceID: workspaceID,
					Status:      domain.ConfigStatusActive,
				}, nil
			},
		},
		ConfigWrite: &mockConfigWrite{
			disableFn: func(ctx context.Context, workspaceID, webhookID string, disabledAt time.Time) error {
				return nil
			},
		},
		AccessChecker: &mockAccessChecker{
			requirePermissionFn: func(ctx context.Context, workspaceID, userID, permission string) error {
				return nil
			},
		},
		DeliveryRead:  &mockDeliveryRead{},
		DeliveryWrite: &mockDeliveryWrite{},
		AttemptRead:   &mockAttemptRead{},
		AttemptWrite:  &mockAttemptWrite{},
		Deliverer: &mockDeliverer{
			deliverFn: func(ctx context.Context, req ports.DeliveryHTTPRequest) (ports.DeliveryHTTPResponse, error) {
				return ports.DeliveryHTTPResponse{}, nil
			},
		},
		IDGen:     fixedIDGen,
		Clock:     fixedClock,
		Logger:    slog.Default(),
		TxManager: &mockTxManager{
			runInTxFn: func(ctx context.Context, fn func(context.Context) error) error {
				return fn(ctx)
			},
		},
	})

	err := svc.DisableWebhookConfig(context.Background(), DisableConfigInput{
		WorkspaceID: "ws-1",
		UserID:      "user-1",
		WebhookID:   "wh-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
