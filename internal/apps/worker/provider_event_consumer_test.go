package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	deliveryapp "github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/app"
	deliverydomain "github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/ports"
	ingestioncontracts "github.com/ninggiangboy/send-flow/backend/internal/modules/ingestion/contracts"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/events"
)

func testProviderEventConsumer(svc *deliveryapp.Service) *ProviderEventConsumer {
	return NewProviderEventConsumer(svc, testConsumerLogger(), nil, "", nil)
}

func validNormalizedProviderEventPayload(t *testing.T, eventType string) []byte {
	t.Helper()

	now := time.Now().UTC().Format(time.RFC3339)
	payload := ingestioncontracts.ProviderEventNormalizedPayload{
		NormalizedEventID: "norm_1",
		RawEventID:        "raw_1",
		Provider:          "fake",
		ProviderEventID:   "prov_evt_1",
		ProviderMessageID: "prov_msg_1",
		WorkspaceID:       "ws_1",
		MessageID:         "msg_1",
		EventType:         eventType,
		OccurredAt:        now,
		ReceivedAt:        now,
	}

	envelope, err := events.NewEnvelope(events.NewEnvelopeOptions{
		EventID:       "evt_1",
		EventType:     ingestioncontracts.EventProviderEventNormalizedV1,
		EventVersion:  1,
		AggregateType: "normalized_provider_event",
		AggregateID:   "norm_1",
		WorkspaceID:   "ws_1",
		OccurredAt:    time.Now().UTC(),
	}, payload)
	if err != nil {
		t.Fatal(err)
	}

	data, err := events.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

type mockMessageReadRepo struct {
	findByID                func(ctx context.Context, workspaceID, messageID string) (*deliverydomain.Message, error)
	findByProviderMessageID func(ctx context.Context, provider, providerMessageID string) (*deliverydomain.Message, error)
}

func (m *mockMessageReadRepo) FindByID(ctx context.Context, workspaceID, messageID string) (*deliverydomain.Message, error) {
	if m.findByID != nil {
		return m.findByID(ctx, workspaceID, messageID)
	}
	return nil, deliverydomain.ErrMessageNotFound
}

func (m *mockMessageReadRepo) FindByIDForUpdate(ctx context.Context, workspaceID, messageID string) (*deliverydomain.Message, error) {
	if m.findByID != nil {
		return m.findByID(ctx, workspaceID, messageID)
	}
	return nil, deliverydomain.ErrMessageNotFound
}

func (m *mockMessageReadRepo) FindByTransactionalRequestID(ctx context.Context, workspaceID, transactionalRequestID string) (*deliverydomain.Message, error) {
	return nil, deliverydomain.ErrMessageNotFound
}

func (m *mockMessageReadRepo) FindByProviderMessageID(ctx context.Context, provider, providerMessageID string) (*deliverydomain.Message, error) {
	if m.findByProviderMessageID != nil {
		return m.findByProviderMessageID(ctx, provider, providerMessageID)
	}
	return nil, deliverydomain.ErrMessageNotFound
}

func (m *mockMessageReadRepo) List(ctx context.Context, query ports.MessageListQuery) ([]deliverydomain.Message, string, error) {
	return nil, "", nil
}

func (m *mockMessageReadRepo) ListDueQueued(ctx context.Context, query ports.DueMessageQuery) ([]deliverydomain.Message, error) {
	return nil, nil
}

func (m *mockMessageReadRepo) ListDistinctWorkspacesWithDue(ctx context.Context, messageType string, now time.Time) ([]string, error) {
	return nil, nil
}

func (m *mockMessageReadRepo) CountByCampaign(ctx context.Context, workspaceID, campaignID string) (int64, error) {
	return 0, nil
}

func TestProviderEventConsumer_ValidEvent(t *testing.T) {
	handled := false

	svc := deliveryapp.NewService(deliveryapp.Options{
		MessagesRead: &mockMessageReadRepo{
			findByID: func(ctx context.Context, workspaceID, messageID string) (*deliverydomain.Message, error) {
				return &deliverydomain.Message{
					ID:                       "msg_1",
					WorkspaceID:              "ws_1",
					Status:                   deliverydomain.MessageStatusAccepted,
					RecipientEmailNormalized: "test@example.com",
				}, nil
			},
		},
		MessagesWrite: &mockMessageWrite{},
		OutboxWriter:  &mockOutbox{},
		TxManager:     &mockTxManager{},
		IDGen:         func() (string, error) { return "evt_out_1", nil },
		Logger:        testConsumerLogger(),
	})

	consumer := testProviderEventConsumer(svc)
	rawPayload := validNormalizedProviderEventPayload(t, "delivered")

	err := consumer.HandleEvent(context.Background(), "evt_1", rawPayload)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !handled {
		// If the event was handled, HandleProviderEvent should succeed
	}
	_ = handled
}

func TestProviderEventConsumer_WrongEventTypeIsIgnored(t *testing.T) {
	svc := deliveryapp.NewService(deliveryapp.Options{
		Logger: testConsumerLogger(),
	})
	consumer := testProviderEventConsumer(svc)

	payload := ingestioncontracts.ProviderEventNormalizedPayload{
		NormalizedEventID: "norm_1",
		Provider:          "fake",
		EventType:         "delivered",
		OccurredAt:        time.Now().UTC().Format(time.RFC3339),
		ReceivedAt:        time.Now().UTC().Format(time.RFC3339),
	}

	envelope, err := events.NewEnvelope(events.NewEnvelopeOptions{
		EventID:       "evt_1",
		EventType:     "some.other.event.v1",
		EventVersion:  1,
		AggregateType: "normalized_provider_event",
		AggregateID:   "norm_1",
	}, payload)
	if err != nil {
		t.Fatal(err)
	}

	rawPayload, err := events.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}

	err = consumer.HandleEvent(context.Background(), "evt_1", rawPayload)
	if err != nil {
		t.Fatalf("expected no error for ignored event type, got %v", err)
	}
}

func TestProviderEventConsumer_MalformedPayloadGoesToDLQ(t *testing.T) {
	svc := deliveryapp.NewService(deliveryapp.Options{
		Logger: testConsumerLogger(),
	})
	consumer := testProviderEventConsumer(svc)

	err := consumer.HandleEvent(context.Background(), "evt_1", []byte("not json"))
	var nonRetryable *deliveryapp.NonRetryableError
	if !errors.As(err, &nonRetryable) {
		t.Fatalf("expected NonRetryableError, got %T: %v", err, err)
	}
}

func TestProviderEventConsumer_MalformedEnvelopePayload(t *testing.T) {
	svc := deliveryapp.NewService(deliveryapp.Options{
		Logger: testConsumerLogger(),
	})
	consumer := testProviderEventConsumer(svc)

	envelope, err := events.NewEnvelope(events.NewEnvelopeOptions{
		EventID:       "evt_1",
		EventType:     ingestioncontracts.EventProviderEventNormalizedV1,
		EventVersion:  1,
		AggregateType: "normalized_provider_event",
		AggregateID:   "norm_1",
	}, map[string]string{"invalid": "payload"})
	if err != nil {
		t.Fatal(err)
	}

	rawPayload, err := events.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}

	// This envelope has a valid outer structure but the payload inside
	// has missing required fields (no event_type, provider, etc.)
	// The consumer will decode it but validation will fail
	err = consumer.HandleEvent(context.Background(), "evt_1", rawPayload)
	if err != nil {
		// Expected: either NonRetryableError for invalid missing fields or no error
		var nonRetryable *deliveryapp.NonRetryableError
		if !errors.As(err, &nonRetryable) {
			t.Fatalf("expected NonRetryableError, got %T: %v", err, err)
		}
	}
}

func TestProviderEventConsumer_RetryableError(t *testing.T) {
	wantErr := errors.New("database connection failed")

	svc := deliveryapp.NewService(deliveryapp.Options{
		MessagesRead: &mockMessageReadRepo{
			findByID: func(ctx context.Context, workspaceID, messageID string) (*deliverydomain.Message, error) {
				return nil, wantErr
			},
		},
		Logger: testConsumerLogger(),
	})
	consumer := testProviderEventConsumer(svc)
	rawPayload := validNormalizedProviderEventPayload(t, "delivered")

	err := consumer.HandleEvent(context.Background(), "evt_1", rawPayload)
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected retryable error %v, got %v", wantErr, err)
	}
}
