package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	deliveryapp "github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/app"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/contracts"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/ports"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/events"
)

func testConsumer(svc *deliveryapp.Service) *CampaignScheduledConsumer {
	return NewCampaignScheduledConsumer(svc, testConsumerLogger(), nil, "", nil)
}

type mockCampaignReader struct {
	countCandidates func(ctx context.Context, workspaceID, campaignID string) (int64, error)
	listCandidates  func(ctx context.Context, workspaceID, campaignID string, limit int, cursor string) ([]ports.CampaignCandidate, string, error)
}

func (m *mockCampaignReader) CountCandidates(ctx context.Context, workspaceID, campaignID string) (int64, error) {
	return m.countCandidates(ctx, workspaceID, campaignID)
}

func (m *mockCampaignReader) ListCandidates(ctx context.Context, workspaceID, campaignID string, limit int, cursor string) ([]ports.CampaignCandidate, string, error) {
	return m.listCandidates(ctx, workspaceID, campaignID, limit, cursor)
}

type mockMessageWrite struct {
	createMany func(ctx context.Context, messages []domain.Message) ([]string, error)
}

func (m *mockMessageWrite) CreateMany(ctx context.Context, messages []domain.Message) ([]string, error) {
	if m.createMany != nil {
		return m.createMany(ctx, messages)
	}
	return nil, nil
}

func (m *mockMessageWrite) Update(ctx context.Context, message domain.Message) error { return nil }

func (m *mockMessageWrite) MarkProcessing(ctx context.Context, workspaceID, messageID string, now time.Time) error {
	return nil
}

func (m *mockMessageWrite) MarkAccepted(ctx context.Context, message domain.Message) error {
	return nil
}
func (m *mockMessageWrite) MarkDelivered(ctx context.Context, message domain.Message) error {
	return nil
}
func (m *mockMessageWrite) MarkFailed(ctx context.Context, message domain.Message) error { return nil }

type mockOutbox struct {
	save func(ctx context.Context, event ports.OutboxEvent) error
}

func (m *mockOutbox) Save(ctx context.Context, event ports.OutboxEvent) error {
	if m.save != nil {
		return m.save(ctx, event)
	}
	return nil
}

type mockTxManager struct {
	runInTransaction func(ctx context.Context, fn func(ctx context.Context) error) error
}

func (m *mockTxManager) RunInTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	if m.runInTransaction != nil {
		return m.runInTransaction(ctx, fn)
	}
	return fn(ctx)
}

func testConsumerLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func validCampaignScheduledPayload(t *testing.T) []byte {
	t.Helper()

	payload := contracts.CampaignScheduledPayload{
		CampaignID:        "camp-1",
		WorkspaceID:       "ws-1",
		TemplateID:        "tpl-1",
		TemplateVersionID: "tv-1",
		SenderDomainID:    "sd-1",
		MessageType:       "marketing",
		ScheduledAt:       time.Now().UTC().Format(time.RFC3339),
	}

	envelope, err := events.NewEnvelope(events.NewEnvelopeOptions{
		EventID:       "evt-1",
		EventType:     contracts.EventCampaignScheduledV1,
		EventVersion:  1,
		AggregateType: "campaign",
		AggregateID:   "camp-1",
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

func TestCampaignScheduledConsumer_ValidEvent(t *testing.T) {
	mockCamp := &mockCampaignReader{
		countCandidates: func(ctx context.Context, ws, camp string) (int64, error) {
			return 1, nil
		},
		listCandidates: func(ctx context.Context, ws, camp string, limit int, cursor string) ([]ports.CampaignCandidate, string, error) {
			return []ports.CampaignCandidate{
				{
					ID:                "cand-1",
					WorkspaceID:       "ws-1",
					CampaignID:        "camp-1",
					ContactID:         "contact-1",
					EmailNormalized:   "test@example.com",
					RecipientSnapshot: json.RawMessage(`{"contact_id":"contact-1","email":"test@example.com","email_normalized":"test@example.com"}`),
				},
			}, "", nil
		},
	}
	mockWrite := &mockMessageWrite{}
	mockOut := &mockOutbox{}
	mockTx := &mockTxManager{}

	idCounter := 0
	svc := deliveryapp.NewService(deliveryapp.Options{
		CampaignReader: mockCamp,
		MessagesWrite:  mockWrite,
		OutboxWriter:   mockOut,
		TxManager:      mockTx,
		IDGen: func() (string, error) {
			idCounter++
			return fmt.Sprintf("id-%d", idCounter), nil
		},
		Logger: testConsumerLogger(),
	})

	consumer := testConsumer(svc)
	rawPayload := validCampaignScheduledPayload(t)

	err := consumer.HandleEvent(context.Background(), "evt-1", rawPayload)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestCampaignScheduledConsumer_NonRetryableWrongEventType(t *testing.T) {
	svc := deliveryapp.NewService(deliveryapp.Options{
		Logger: testConsumerLogger(),
	})
	consumer := testConsumer(svc)

	payload := contracts.CampaignScheduledPayload{
		CampaignID:        "camp-1",
		WorkspaceID:       "ws-1",
		TemplateID:        "tpl-1",
		TemplateVersionID: "tv-1",
		SenderDomainID:    "sd-1",
		MessageType:       "marketing",
		ScheduledAt:       time.Now().UTC().Format(time.RFC3339),
	}

	envelope, err := events.NewEnvelope(events.NewEnvelopeOptions{
		EventID:       "evt-1",
		EventType:     "some.other.event.v1",
		EventVersion:  1,
		AggregateType: "campaign",
		AggregateID:   "camp-1",
		WorkspaceID:   "ws-1",
		OccurredAt:    time.Now().UTC(),
	}, payload)
	if err != nil {
		t.Fatal(err)
	}

	rawPayload, err := events.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}

	err = consumer.HandleEvent(context.Background(), "evt-1", rawPayload)
	var nonRetryable *deliveryapp.NonRetryableError
	if !errors.As(err, &nonRetryable) {
		t.Fatalf("expected NonRetryableError, got %v", err)
	}
}

func TestCampaignScheduledConsumer_NonRetryableInvalidPayload(t *testing.T) {
	svc := deliveryapp.NewService(deliveryapp.Options{
		Logger: testConsumerLogger(),
	})
	consumer := testConsumer(svc)

	err := consumer.HandleEvent(context.Background(), "evt-1", []byte("not json"))
	var nonRetryable *deliveryapp.NonRetryableError
	if !errors.As(err, &nonRetryable) {
		t.Fatalf("expected NonRetryableError, got %v", err)
	}
}

func TestCampaignScheduledConsumer_RetryableError(t *testing.T) {
	wantErr := errors.New("database connection failed")
	mockCamp := &mockCampaignReader{
		countCandidates: func(ctx context.Context, ws, camp string) (int64, error) {
			return 0, wantErr
		},
	}
	mockTx := &mockTxManager{}
	svc := deliveryapp.NewService(deliveryapp.Options{
		CampaignReader: mockCamp,
		TxManager:      mockTx,
		Logger:         testConsumerLogger(),
	})
	consumer := testConsumer(svc)
	rawPayload := validCampaignScheduledPayload(t)

	err := consumer.HandleEvent(context.Background(), "evt-1", rawPayload)
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected retryable error %v, got %v", wantErr, err)
	}
}
