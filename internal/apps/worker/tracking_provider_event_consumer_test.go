package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	ingestioncontracts "github.com/ninggiangboy/send-flow/backend/internal/modules/ingestion/contracts"
	trackingapp "github.com/ninggiangboy/send-flow/backend/internal/modules/tracking/app"
	trackingdomain "github.com/ninggiangboy/send-flow/backend/internal/modules/tracking/domain"
	trackingports "github.com/ninggiangboy/send-flow/backend/internal/modules/tracking/ports"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/events"
	platformerrors "github.com/ninggiangboy/send-flow/backend/internal/platform/retryable"
)

type mockTrackingMsgResolver struct {
	findByID                func(ctx context.Context, workspaceID, messageID string) (string, string, string, string, string, string, error)
	findByProviderMessageID func(ctx context.Context, provider, providerMessageID string) (string, string, string, error)
}

func (m *mockTrackingMsgResolver) FindByID(ctx context.Context, workspaceID, messageID string) (string, string, string, string, string, string, error) {
	if m.findByID != nil {
		return m.findByID(ctx, workspaceID, messageID)
	}
	return "", "", "", "", "", "", nil
}

func (m *mockTrackingMsgResolver) FindByProviderMessageID(ctx context.Context, provider, providerMessageID string) (string, string, string, error) {
	if m.findByProviderMessageID != nil {
		return m.findByProviderMessageID(ctx, provider, providerMessageID)
	}
	return "", "", "", nil
}

type mockTrackingEventWrite struct {
	create func(ctx context.Context, event trackingdomain.TrackingEvent) error
}

func (m *mockTrackingEventWrite) Create(ctx context.Context, event trackingdomain.TrackingEvent) error {
	if m.create != nil {
		return m.create(ctx, event)
	}
	return nil
}

func (m *mockTrackingEventWrite) FindBySourceEvent(ctx context.Context, source, sourceEventID, eventType string) (*trackingdomain.TrackingEvent, error) {
	return nil, nil
}

func (m *mockTrackingEventWrite) ListByMessage(ctx context.Context, workspaceID, messageID string, limit int, cursor string) ([]trackingdomain.TrackingEvent, string, error) {
	return nil, "", nil
}

type mockTrackingEventRead struct {
	findBySourceEvent func(ctx context.Context, source, sourceEventID, eventType string) (*trackingdomain.TrackingEvent, error)
	listByMessage     func(ctx context.Context, workspaceID, messageID string, limit int, cursor string) ([]trackingdomain.TrackingEvent, string, error)
}

func (m *mockTrackingEventRead) FindBySourceEvent(ctx context.Context, source, sourceEventID, eventType string) (*trackingdomain.TrackingEvent, error) {
	if m.findBySourceEvent != nil {
		return m.findBySourceEvent(ctx, source, sourceEventID, eventType)
	}
	return nil, nil
}

func (m *mockTrackingEventRead) ListByMessage(ctx context.Context, workspaceID, messageID string, limit int, cursor string) ([]trackingdomain.TrackingEvent, string, error) {
	return nil, "", nil
}

type mockTrackingLinkRead struct{}

func (m *mockTrackingLinkRead) FindByID(ctx context.Context, trackingID string) (*trackingdomain.TrackingLink, error) {
	return nil, trackingdomain.ErrTrackingLinkNotFound
}

func (m *mockTrackingLinkRead) ListByMessage(ctx context.Context, workspaceID, messageID string) ([]trackingdomain.TrackingLink, error) {
	return nil, nil
}

type mockTrackingLinkWrite struct{}

func (m *mockTrackingLinkWrite) Create(ctx context.Context, link trackingdomain.TrackingLink) error {
	return nil
}

type mockTrackingOutbox struct {
	save func(ctx context.Context, event trackingports.OutboxEvent) error
}

func (m *mockTrackingOutbox) Save(ctx context.Context, event trackingports.OutboxEvent) error {
	if m.save != nil {
		return m.save(ctx, event)
	}
	return nil
}

type mockTrackingTxManager struct {
	withinTx func(ctx context.Context, fn func(ctx context.Context) error) error
}

func (m *mockTrackingTxManager) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if m.withinTx != nil {
		return m.withinTx(ctx, fn)
	}
	return fn(ctx)
}

func testTrackingProviderEventConsumer(svc *trackingapp.Service) *TrackingProviderEventConsumer {
	return NewTrackingProviderEventConsumer(svc, testConsumerLogger(), nil, "", nil)
}

func validTrackingProviderEventEnvelope(t *testing.T) []byte {
	t.Helper()

	now := time.Now().UTC().Format(time.RFC3339)
	payload := ingestioncontracts.ProviderEventNormalizedPayload{
		NormalizedEventID: "norm-1",
		RawEventID:        "raw-1",
		Provider:          "fake",
		ProviderEventID:   "prov-evt-1",
		ProviderMessageID: "prov-msg-1",
		WorkspaceID:       "ws-1",
		MessageID:         "msg-1",
		EventType:         "opened",
		OccurredAt:        now,
		ReceivedAt:        now,
	}

	envelope, err := events.NewEnvelope(events.NewEnvelopeOptions{
		EventID:       "evt-1",
		EventType:     ingestioncontracts.EventProviderEventNormalizedV1,
		EventVersion:  1,
		AggregateType: "normalized_provider_event",
		AggregateID:   "norm-1",
		WorkspaceID:   "ws-1",
		OccurredAt:    time.Now().UTC(),
	}, payload)
	if err != nil {
		t.Fatal(err)
	}

	raw, err := events.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestTrackingProviderEventConsumer_ValidEvent(t *testing.T) {
	svc := trackingapp.NewService(trackingapp.Options{
		MessageResolver: &mockTrackingMsgResolver{
			findByID: func(ctx context.Context, workspaceID, messageID string) (string, string, string, string, string, string, error) {
				return "msg-1", "ws-1", "camp-1", "fake", "prov-msg-1", "test@example.com", nil
			},
		},
		EventWriteRepo: &mockTrackingEventWrite{},
		EventReadRepo:  &mockTrackingEventRead{},
		LinkReadRepo:   &mockTrackingLinkRead{},
		OutboxWriter:   &mockTrackingOutbox{},
		TxManager:      &mockTrackingTxManager{},
		IDGen:          func() (string, error) { return "id-1", nil },
		Logger:         testConsumerLogger(),
	})
	consumer := testTrackingProviderEventConsumer(svc)
	rawPayload := validTrackingProviderEventEnvelope(t)

	err := consumer.HandleEvent(context.Background(), "evt-1", rawPayload)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestTrackingProviderEventConsumer_WrongEventTypeIsIgnored(t *testing.T) {
	svc := trackingapp.NewService(trackingapp.Options{
		Logger: testConsumerLogger(),
	})
	consumer := testTrackingProviderEventConsumer(svc)

	now := time.Now().UTC().Format(time.RFC3339)
	payload := ingestioncontracts.ProviderEventNormalizedPayload{
		NormalizedEventID: "norm-1",
		Provider:          "fake",
		EventType:         "opened",
		OccurredAt:        now,
		ReceivedAt:        now,
	}

	envelope, err := events.NewEnvelope(events.NewEnvelopeOptions{
		EventID:       "evt-1",
		EventType:     "some.other.event.v1",
		EventVersion:  1,
		AggregateType: "normalized_provider_event",
		AggregateID:   "norm-1",
	}, payload)
	if err != nil {
		t.Fatal(err)
	}

	rawPayload, err := events.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}

	err = consumer.HandleEvent(context.Background(), "evt-1", rawPayload)
	if err != nil {
		t.Fatalf("expected no error for wrong event type, got %v", err)
	}
}

func TestTrackingProviderEventConsumer_MalformedPayload(t *testing.T) {
	svc := trackingapp.NewService(trackingapp.Options{
		Logger: testConsumerLogger(),
	})
	consumer := testTrackingProviderEventConsumer(svc)

	err := consumer.HandleEvent(context.Background(), "evt-1", []byte("not json"))
	var nonRetryable *platformerrors.NonRetryableError
	if !errors.As(err, &nonRetryable) {
		t.Fatalf("expected NonRetryableError, got %T: %v", err, err)
	}
}

func TestTrackingProviderEventConsumer_RetryableError(t *testing.T) {
	wantErr := errors.New("database connection failed")
	svc := trackingapp.NewService(trackingapp.Options{
		MessageResolver: &mockTrackingMsgResolver{
			findByID: func(ctx context.Context, workspaceID, messageID string) (string, string, string, string, string, string, error) {
				return "msg-1", "ws-1", "camp-1", "fake", "prov-msg-1", "test@example.com", nil
			},
		},
		EventWriteRepo: &mockTrackingEventWrite{},
		EventReadRepo: &mockTrackingEventRead{
			findBySourceEvent: func(ctx context.Context, source, sourceEventID, eventType string) (*trackingdomain.TrackingEvent, error) {
				return nil, wantErr
			},
		},
		LinkReadRepo: &mockTrackingLinkRead{},
		TxManager:    &mockTrackingTxManager{},
		OutboxWriter: &mockTrackingOutbox{},
		IDGen:        func() (string, error) { return "id-1", nil },
		Logger:       testConsumerLogger(),
	})
	consumer := testTrackingProviderEventConsumer(svc)
	rawPayload := validTrackingProviderEventEnvelope(t)

	err := consumer.HandleEvent(context.Background(), "evt-1", rawPayload)
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected retryable error %v, got %v", wantErr, err)
	}
}
