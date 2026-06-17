package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	analyticsapp "github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/app"
	analyticsdomain "github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/events"
	platformerrors "github.com/ninggiangboy/send-flow/backend/internal/platform/retryable"
)

type mockAnalyticsFactRepo struct {
	findBySourceEventID func(ctx context.Context, sourceEventID string) (*analyticsdomain.EmailEventFact, error)
	create              func(ctx context.Context, fact analyticsdomain.EmailEventFact) error
}

func (m *mockAnalyticsFactRepo) FindBySourceEventID(ctx context.Context, sourceEventID string) (*analyticsdomain.EmailEventFact, error) {
	if m.findBySourceEventID != nil {
		return m.findBySourceEventID(ctx, sourceEventID)
	}
	return nil, analyticsdomain.ErrAnalyticsProjectionNotFound
}

func (m *mockAnalyticsFactRepo) Create(ctx context.Context, fact analyticsdomain.EmailEventFact) error {
	if m.create != nil {
		return m.create(ctx, fact)
	}
	return nil
}

func testAnalyticsEventConsumer(svc *analyticsapp.Service, registry *analyticsapp.MapperRegistry) *AnalyticsEventConsumer {
	return NewAnalyticsEventConsumer(svc, registry, testConsumerLogger(), nil, "", nil)
}

func validAnalyticsEnvelope(t *testing.T, eventType string) []byte {
	t.Helper()

	envelope, err := events.NewEnvelope(events.NewEnvelopeOptions{
		EventID:       "evt-1",
		EventType:     eventType,
		EventVersion:  1,
		AggregateType: "test",
		AggregateID:   "agg-1",
		WorkspaceID:   "ws-1",
		OccurredAt:    time.Now().UTC(),
	}, map[string]string{"test": "data"})
	if err != nil {
		t.Fatal(err)
	}

	raw, err := events.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestAnalyticsEventConsumer_ValidEvent(t *testing.T) {
	registry := analyticsapp.NewMapperRegistry()
	registry.Register("test.event.v1", func(envelope events.Envelope) (*analyticsapp.MappedEvent, error) {
		return &analyticsapp.MappedEvent{
			Input: analyticsapp.IngestEmailEventFactInput{
				SourceEventID:   envelope.EventID,
				SourceEventType: envelope.EventType,
				WorkspaceID:     envelope.WorkspaceID,
				CampaignID:      "camp-1",
				MessageID:       "msg-1",
				CanonicalType:   "delivered",
				OccurredAt:      time.Now().UTC(),
				ReceivedAt:      time.Now().UTC(),
			},
		}, nil
	})

	svc := analyticsapp.NewService(analyticsapp.Options{
		FactRepo: &mockAnalyticsFactRepo{},
		Clock:    time.Now,
		Logger:   testConsumerLogger(),
	})
	consumer := testAnalyticsEventConsumer(svc, registry)
	rawPayload := validAnalyticsEnvelope(t, "test.event.v1")

	err := consumer.HandleEvent(context.Background(), "evt-1", rawPayload)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestAnalyticsEventConsumer_WrongEventTypeIsIgnored(t *testing.T) {
	registry := analyticsapp.NewMapperRegistry()
	registry.Register("test.event.v1", func(envelope events.Envelope) (*analyticsapp.MappedEvent, error) {
		return &analyticsapp.MappedEvent{
			Input: analyticsapp.IngestEmailEventFactInput{
				WorkspaceID: "ws-1",
			},
		}, nil
	})

	svc := analyticsapp.NewService(analyticsapp.Options{
		Logger: testConsumerLogger(),
	})
	consumer := testAnalyticsEventConsumer(svc, registry)
	rawPayload := validAnalyticsEnvelope(t, "some.other.event.v1")

	err := consumer.HandleEvent(context.Background(), "evt-1", rawPayload)
	if err != nil {
		t.Fatalf("expected no error for unsupported event type, got %v", err)
	}
}

func TestAnalyticsEventConsumer_MalformedPayload(t *testing.T) {
	registry := analyticsapp.NewMapperRegistry()
	svc := analyticsapp.NewService(analyticsapp.Options{
		Logger: testConsumerLogger(),
	})
	consumer := testAnalyticsEventConsumer(svc, registry)

	err := consumer.HandleEvent(context.Background(), "evt-1", []byte("not json"))
	var nonRetryable *platformerrors.NonRetryableError
	if !errors.As(err, &nonRetryable) {
		t.Fatalf("expected NonRetryableError, got %T: %v", err, err)
	}
}

func TestAnalyticsEventConsumer_ClickHouseFailureDoesNotBlockIngestion(t *testing.T) {
	registry := analyticsapp.NewMapperRegistry()
	registry.Register("test.event.v1", func(envelope events.Envelope) (*analyticsapp.MappedEvent, error) {
		return &analyticsapp.MappedEvent{
			Input: analyticsapp.IngestEmailEventFactInput{
				SourceEventID:   envelope.EventID,
				SourceEventType: envelope.EventType,
				WorkspaceID:     envelope.WorkspaceID,
				CampaignID:      "camp-1",
				MessageID:       "msg-1",
				CanonicalType:   "delivered",
				OccurredAt:      time.Now().UTC(),
				ReceivedAt:      time.Now().UTC(),
			},
		}, nil
	})

	svc := analyticsapp.NewService(analyticsapp.Options{
		FactRepo: &mockAnalyticsFactRepo{
			findBySourceEventID: func(_ context.Context, _ string) (*analyticsdomain.EmailEventFact, error) {
				return nil, analyticsdomain.ErrAnalyticsProjectionNotFound
			},
			create: func(_ context.Context, _ analyticsdomain.EmailEventFact) error {
				return nil
			},
		},
		Clock:  time.Now,
		Logger: testConsumerLogger(),
	})
	consumer := testAnalyticsEventConsumer(svc, registry)
	rawPayload := validAnalyticsEnvelope(t, "test.event.v1")

	err := consumer.HandleEvent(context.Background(), "evt-1", rawPayload)
	if err != nil {
		t.Fatalf("expected no error since ClickHouse failure does not block ingestion, got %v", err)
	}
}

func TestAnalyticsEventConsumer_EventMissingWorkspaceID(t *testing.T) {
	registry := analyticsapp.NewMapperRegistry()
	registry.Register("test.event.v1", func(envelope events.Envelope) (*analyticsapp.MappedEvent, error) {
		return &analyticsapp.MappedEvent{
			Input: analyticsapp.IngestEmailEventFactInput{
				WorkspaceID: "",
			},
		}, nil
	})

	svc := analyticsapp.NewService(analyticsapp.Options{
		Logger: testConsumerLogger(),
	})
	consumer := testAnalyticsEventConsumer(svc, registry)

	envelope, err := events.NewEnvelope(events.NewEnvelopeOptions{
		EventID:       "evt-1",
		EventType:     "test.event.v1",
		EventVersion:  1,
		AggregateType: "test",
		AggregateID:   "agg-1",
		OccurredAt:    time.Now().UTC(),
	}, map[string]string{"test": "data"})
	if err != nil {
		t.Fatal(err)
	}

	rawPayload, err := events.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}

	err = consumer.HandleEvent(context.Background(), "evt-1", rawPayload)
	if err != nil {
		t.Fatalf("expected no error for missing workspace_id, got %v", err)
	}
}
