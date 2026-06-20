package worker

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	webhooksapp "github.com/ninggiangboy/send-flow/backend/internal/modules/webhooks/app"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/webhooks/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/webhooks/ports"
)

type webhookProcessorDeliveryRead struct {
	deliveries map[string]domain.WebhookDelivery
}

func (m *webhookProcessorDeliveryRead) FindByID(ctx context.Context, workspaceID, deliveryID string) (*domain.WebhookDelivery, error) {
	delivery := m.deliveries[deliveryID]
	return &delivery, nil
}

func (m *webhookProcessorDeliveryRead) FindByWebhookAndEvent(ctx context.Context, webhookID, sourceEventID string) (*domain.WebhookDelivery, error) {
	return nil, nil
}

func (m *webhookProcessorDeliveryRead) ListByWorkspace(ctx context.Context, workspaceID string, filter ports.DeliveryFilter) ([]domain.WebhookDelivery, string, error) {
	return nil, "", nil
}

type webhookProcessorDeliveryWrite struct {
	claimed    []domain.WebhookDelivery
	deliveries map[string]domain.WebhookDelivery
}

func (m *webhookProcessorDeliveryWrite) Create(ctx context.Context, delivery domain.WebhookDelivery) error {
	return nil
}

func (m *webhookProcessorDeliveryWrite) MarkDelivering(ctx context.Context, deliveryID string, now time.Time) error {
	return nil
}

func (m *webhookProcessorDeliveryWrite) MarkSucceeded(ctx context.Context, deliveryID string, result domain.DeliveryResult) error {
	return nil
}

func (m *webhookProcessorDeliveryWrite) MarkFailed(ctx context.Context, deliveryID string, result domain.DeliveryResult) error {
	return nil
}

func (m *webhookProcessorDeliveryWrite) ScheduleRetry(ctx context.Context, deliveryID string, nextAttemptAt time.Time) error {
	return nil
}

func (m *webhookProcessorDeliveryWrite) ClaimPendingDeliveries(ctx context.Context, limit int, now time.Time) ([]domain.WebhookDelivery, error) {
	return m.claimed, nil
}
func (m *webhookProcessorDeliveryWrite) FindByID(ctx context.Context, workspaceID, deliveryID string) (*domain.WebhookDelivery, error) {
	if m.deliveries != nil {
		if d, ok := m.deliveries[deliveryID]; ok {
			return &d, nil
		}
	}
	return nil, nil
}
func (m *webhookProcessorDeliveryWrite) FindByWebhookAndEvent(ctx context.Context, webhookID, sourceEventID string) (*domain.WebhookDelivery, error) {
	return nil, nil
}
func (m *webhookProcessorDeliveryWrite) ListByWorkspace(ctx context.Context, workspaceID string, filter ports.DeliveryFilter) ([]domain.WebhookDelivery, string, error) {
	return nil, "", nil
}

type webhookProcessorConfigRead struct{}

func (m *webhookProcessorConfigRead) FindByID(ctx context.Context, workspaceID, webhookID string) (*domain.WebhookConfig, error) {
	return &domain.WebhookConfig{
		ID:          webhookID,
		WorkspaceID: workspaceID,
		Status:      domain.ConfigStatusActive,
		TargetURL:   "https://example.com/" + webhookID,
		SecretHash:  "secret",
	}, nil
}

func (m *webhookProcessorConfigRead) ListByWorkspace(ctx context.Context, workspaceID string) ([]domain.WebhookConfig, error) {
	return nil, nil
}

func (m *webhookProcessorConfigRead) ListSubscribed(ctx context.Context, workspaceID, eventType string) ([]domain.WebhookConfig, error) {
	return nil, nil
}

type webhookProcessorConfigWrite struct {
	*webhookProcessorConfigRead
}

func (m *webhookProcessorConfigWrite) Create(ctx context.Context, config domain.WebhookConfig) error {
	return nil
}

func (m *webhookProcessorConfigWrite) Update(ctx context.Context, config domain.WebhookConfig) error {
	return nil
}

func (m *webhookProcessorConfigWrite) Disable(ctx context.Context, workspaceID, webhookID string, disabledAt time.Time) error {
	return nil
}

type webhookProcessorAttemptWrite struct{}

func (m *webhookProcessorAttemptWrite) Create(ctx context.Context, attempt domain.WebhookDeliveryAttempt) error {
	return nil
}
func (m *webhookProcessorAttemptWrite) ListByDelivery(ctx context.Context, deliveryID string) ([]domain.WebhookDeliveryAttempt, error) {
	return nil, nil
}

type webhookProcessorTx struct{}

func (m *webhookProcessorTx) WithinTx(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

type webhookProcessorDeliverer struct{}

func (m *webhookProcessorDeliverer) Deliver(ctx context.Context, req ports.DeliveryHTTPRequest) (ports.DeliveryHTTPResponse, error) {
	switch req.URL {
	case "https://example.com/wh_success":
		return ports.DeliveryHTTPResponse{StatusCode: 200, DurationMs: 10}, nil
	case "https://example.com/wh_retry":
		return ports.DeliveryHTTPResponse{StatusCode: 500, DurationMs: 20, Error: "server error"}, nil
	default:
		return ports.DeliveryHTTPResponse{StatusCode: 400, DurationMs: 30, Error: "bad request"}, nil
	}
}

func TestDueWebhookDeliveryProcessor_RecordsPipelineOutcomes(t *testing.T) {
	deliveries := []domain.WebhookDelivery{
		{
			ID:               "del_success",
			WorkspaceID:      "ws_1",
			WebhookID:        "wh_success",
			TargetURL:        "https://example.com/wh_success",
			SourceEventID:    "evt_success",
			SourceEventType:  "delivery.message.delivered.v1",
			AttemptCount:     1,
			EventPayloadJSON: map[string]any{"event": "success"},
		},
		{
			ID:               "del_retry",
			WorkspaceID:      "ws_1",
			WebhookID:        "wh_retry",
			TargetURL:        "https://example.com/wh_retry",
			SourceEventID:    "evt_retry",
			SourceEventType:  "delivery.message.delivered.v1",
			AttemptCount:     1,
			EventPayloadJSON: map[string]any{"event": "retry"},
		},
		{
			ID:               "del_failure",
			WorkspaceID:      "ws_1",
			WebhookID:        "wh_failure",
			TargetURL:        "https://example.com/wh_failure",
			SourceEventID:    "evt_failure",
			SourceEventType:  "delivery.message.delivered.v1",
			AttemptCount:     1,
			EventPayloadJSON: map[string]any{"event": "failure"},
		},
	}
	deliveryByID := make(map[string]domain.WebhookDelivery, len(deliveries))
	for _, delivery := range deliveries {
		deliveryByID[delivery.ID] = delivery
	}

	idCounter := 0
	svc := webhooksapp.NewService(webhooksapp.Options{
		ConfigRead:    &webhookProcessorConfigRead{},
		ConfigWrite:   &webhookProcessorConfigWrite{&webhookProcessorConfigRead{}},
		DeliveryRead:  &webhookProcessorDeliveryRead{deliveries: deliveryByID},
		DeliveryWrite: &webhookProcessorDeliveryWrite{claimed: deliveries, deliveries: deliveryByID},
		AttemptWrite:  &webhookProcessorAttemptWrite{},
		TxManager:     &webhookProcessorTx{},
		Deliverer:     &webhookProcessorDeliverer{},
		IDGen: func() (string, error) {
			idCounter++
			return fmt.Sprintf("id_%d", idCounter), nil
		},
		Clock:  func() time.Time { return time.Date(2026, 6, 16, 0, 0, 0, 0, time.UTC) },
		Logger: slog.Default(),
	})
	processor := NewDueWebhookDeliveryProcessor(svc, testConsumerLogger(), time.Minute, 3)

	recorded := make(map[string]string)
	processor.SetOperationsRecorder(func(ctx context.Context, source, sourceEventType, operationType, status, workspaceID, errorType, consumer, target string, occurredAt time.Time) {
		recorded[operationType] = status
	})

	processed, err := processor.Poll(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !processed {
		t.Fatal("expected deliveries to be processed")
	}

	want := map[string]string{
		"webhook_succeeded":       "success",
		"webhook_retry_scheduled": "retry",
		"webhook_failed":          "failure",
	}
	for opType, status := range want {
		if recorded[opType] != status {
			t.Fatalf("expected %s=%s, got %#v", opType, status, recorded)
		}
	}
}
