package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	deliverycontracts "github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/contracts"
	webhooksapp "github.com/ninggiangboy/send-flow/backend/internal/modules/webhooks/app"
	webhooksdomain "github.com/ninggiangboy/send-flow/backend/internal/modules/webhooks/domain"
	webhooksports "github.com/ninggiangboy/send-flow/backend/internal/modules/webhooks/ports"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/events"
	platformerrors "github.com/ninggiangboy/send-flow/backend/internal/platform/retryable"
)

type mockWebhookConfigRead struct {
	listSubscribed func(ctx context.Context, workspaceID, eventType string) ([]webhooksdomain.WebhookConfig, error)
}

func (m *mockWebhookConfigRead) FindByID(ctx context.Context, workspaceID, webhookID string) (*webhooksdomain.WebhookConfig, error) {
	return nil, nil
}

func (m *mockWebhookConfigRead) ListByWorkspace(ctx context.Context, workspaceID string) ([]webhooksdomain.WebhookConfig, error) {
	return nil, nil
}

func (m *mockWebhookConfigRead) ListSubscribed(ctx context.Context, workspaceID, eventType string) ([]webhooksdomain.WebhookConfig, error) {
	if m.listSubscribed != nil {
		return m.listSubscribed(ctx, workspaceID, eventType)
	}
	return nil, nil
}

type mockWebhookConfigWrite struct{}

func (m *mockWebhookConfigWrite) Create(ctx context.Context, config webhooksdomain.WebhookConfig) error {
	return nil
}

func (m *mockWebhookConfigWrite) Update(ctx context.Context, config webhooksdomain.WebhookConfig) error {
	return nil
}

func (m *mockWebhookConfigWrite) Disable(ctx context.Context, workspaceID, webhookID string, disabledAt time.Time) error {
	return nil
}

type mockWebhookDeliveryWrite struct{}

func (m *mockWebhookDeliveryWrite) Create(ctx context.Context, delivery webhooksdomain.WebhookDelivery) error {
	return nil
}

func (m *mockWebhookDeliveryWrite) MarkDelivering(ctx context.Context, deliveryID string, now time.Time) error {
	return nil
}

func (m *mockWebhookDeliveryWrite) MarkSucceeded(ctx context.Context, deliveryID string, result webhooksdomain.DeliveryResult) error {
	return nil
}

func (m *mockWebhookDeliveryWrite) MarkFailed(ctx context.Context, deliveryID string, result webhooksdomain.DeliveryResult) error {
	return nil
}

func (m *mockWebhookDeliveryWrite) ScheduleRetry(ctx context.Context, deliveryID string, nextAttemptAt time.Time) error {
	return nil
}

func (m *mockWebhookDeliveryWrite) ClaimPendingDeliveries(ctx context.Context, limit int, now time.Time) ([]webhooksdomain.WebhookDelivery, error) {
	return nil, nil
}

type mockWebhookDeliveryRead struct{}

func (m *mockWebhookDeliveryRead) FindByID(ctx context.Context, workspaceID, deliveryID string) (*webhooksdomain.WebhookDelivery, error) {
	return nil, nil
}

func (m *mockWebhookDeliveryRead) FindByWebhookAndEvent(ctx context.Context, webhookID, sourceEventID string) (*webhooksdomain.WebhookDelivery, error) {
	return nil, nil
}

func (m *mockWebhookDeliveryRead) ListByWorkspace(ctx context.Context, workspaceID string, filter webhooksports.DeliveryFilter) ([]webhooksdomain.WebhookDelivery, string, error) {
	return nil, "", nil
}

type mockWebhookAttemptWrite struct{}

func (m *mockWebhookAttemptWrite) Create(ctx context.Context, attempt webhooksdomain.WebhookDeliveryAttempt) error {
	return nil
}

type mockWebhookAttemptRead struct{}

func (m *mockWebhookAttemptRead) ListByDelivery(ctx context.Context, deliveryID string) ([]webhooksdomain.WebhookDeliveryAttempt, error) {
	return nil, nil
}

type mockWebhookDeliverer struct{}

func (m *mockWebhookDeliverer) Deliver(ctx context.Context, req webhooksports.DeliveryHTTPRequest) (webhooksports.DeliveryHTTPResponse, error) {
	return webhooksports.DeliveryHTTPResponse{}, nil
}

func testWebhookEventConsumer(svc *webhooksapp.Service) *WebhookEventConsumer {
	return NewWebhookEventConsumer(svc, testConsumerLogger(), nil, "", nil)
}

func validDeliveryQueuedEnvelope(t *testing.T) []byte {
	t.Helper()

	envelope, err := events.NewEnvelope(events.NewEnvelopeOptions{
		EventID:       "evt-1",
		EventType:     deliverycontracts.EventDeliveryMessageQueuedV1,
		EventVersion:  1,
		AggregateType: "message",
		AggregateID:   "msg-1",
		WorkspaceID:   "ws-1",
		OccurredAt:    time.Now().UTC(),
	}, map[string]string{"message_id": "msg-1"})
	if err != nil {
		t.Fatal(err)
	}

	raw, err := events.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestWebhookEventConsumer_ValidDeliveryEvent(t *testing.T) {
	svc := webhooksapp.NewService(webhooksapp.Options{
		ConfigRead:    &mockWebhookConfigRead{},
		DeliveryWrite: &mockWebhookDeliveryWrite{},
		Logger:        testConsumerLogger(),
	})
	consumer := testWebhookEventConsumer(svc)
	rawPayload := validDeliveryQueuedEnvelope(t)

	err := consumer.HandleEvent(context.Background(), "evt-1", rawPayload)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestWebhookEventConsumer_WrongEventTypeIsIgnored(t *testing.T) {
	svc := webhooksapp.NewService(webhooksapp.Options{
		Logger: testConsumerLogger(),
	})
	consumer := testWebhookEventConsumer(svc)

	envelope, err := events.NewEnvelope(events.NewEnvelopeOptions{
		EventID:       "evt-1",
		EventType:     "some.other.event.v1",
		EventVersion:  1,
		AggregateType: "message",
		AggregateID:   "msg-1",
		WorkspaceID:   "ws-1",
		OccurredAt:    time.Now().UTC(),
	}, map[string]string{"foo": "bar"})
	if err != nil {
		t.Fatal(err)
	}

	rawPayload, err := events.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}

	err = consumer.HandleEvent(context.Background(), "evt-1", rawPayload)
	if err != nil {
		t.Fatalf("expected no error for unsupported event type, got %v", err)
	}
}

func TestWebhookEventConsumer_MalformedPayload(t *testing.T) {
	svc := webhooksapp.NewService(webhooksapp.Options{
		Logger: testConsumerLogger(),
	})
	consumer := testWebhookEventConsumer(svc)

	err := consumer.HandleEvent(context.Background(), "evt-1", []byte("not json"))
	var nonRetryable *platformerrors.NonRetryableError
	if !errors.As(err, &nonRetryable) {
		t.Fatalf("expected NonRetryableError, got %T: %v", err, err)
	}
}

func TestWebhookEventConsumer_EventMissingWorkspaceID(t *testing.T) {
	svc := webhooksapp.NewService(webhooksapp.Options{
		Logger: testConsumerLogger(),
	})
	consumer := testWebhookEventConsumer(svc)

	envelope, err := events.NewEnvelope(events.NewEnvelopeOptions{
		EventID:       "evt-1",
		EventType:     deliverycontracts.EventDeliveryMessageQueuedV1,
		EventVersion:  1,
		AggregateType: "message",
		AggregateID:   "msg-1",
		OccurredAt:    time.Now().UTC(),
	}, map[string]string{"message_id": "msg-1"})
	if err != nil {
		t.Fatal(err)
	}

	rawPayload, err := events.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}

	err = consumer.HandleEvent(context.Background(), "evt-1", rawPayload)
	var nonRetryable *platformerrors.NonRetryableError
	if !errors.As(err, &nonRetryable) {
		t.Fatalf("expected NonRetryableError for missing workspace_id, got %T: %v", err, err)
	}
}

func TestWebhookEventConsumer_RetryableError(t *testing.T) {
	wantErr := errors.New("database connection failed")
	svc := webhooksapp.NewService(webhooksapp.Options{
		ConfigRead: &mockWebhookConfigRead{
			listSubscribed: func(ctx context.Context, workspaceID, eventType string) ([]webhooksdomain.WebhookConfig, error) {
				return nil, wantErr
			},
		},
		DeliveryWrite: &mockWebhookDeliveryWrite{},
		Logger:        testConsumerLogger(),
	})
	consumer := testWebhookEventConsumer(svc)
	rawPayload := validDeliveryQueuedEnvelope(t)

	err := consumer.HandleEvent(context.Background(), "evt-1", rawPayload)
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected retryable error %v, got %v", wantErr, err)
	}
}
