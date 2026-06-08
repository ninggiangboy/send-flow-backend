package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/contracts"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/ports"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/events"
)

type mockCampaignCandidateReader struct {
	countCandidatesFunc func(ctx context.Context, workspaceID, campaignID string) (int64, error)
	listCandidatesFunc  func(ctx context.Context, workspaceID, campaignID string, limit int, cursor string) ([]ports.CampaignCandidate, string, error)
}

func (m *mockCampaignCandidateReader) CountCandidates(ctx context.Context, workspaceID, campaignID string) (int64, error) {
	return m.countCandidatesFunc(ctx, workspaceID, campaignID)
}

func (m *mockCampaignCandidateReader) ListCandidates(ctx context.Context, workspaceID, campaignID string, limit int, cursor string) ([]ports.CampaignCandidate, string, error) {
	return m.listCandidatesFunc(ctx, workspaceID, campaignID, limit, cursor)
}

type mockMessageWriteRepository struct {
	createManyFunc func(ctx context.Context, messages []domain.Message) ([]string, error)
}

func (m *mockMessageWriteRepository) CreateMany(ctx context.Context, messages []domain.Message) ([]string, error) {
	return m.createManyFunc(ctx, messages)
}

func (m *mockMessageWriteRepository) Update(_ context.Context, _ domain.Message) error {
	return nil
}

func (m *mockMessageWriteRepository) MarkProcessing(_ context.Context, _, _ string, _ time.Time) error {
	return nil
}

func (m *mockMessageWriteRepository) MarkAccepted(_ context.Context, _ domain.Message) error {
	return nil
}

func (m *mockMessageWriteRepository) MarkDelivered(_ context.Context, _ domain.Message) error {
	return nil
}

func (m *mockMessageWriteRepository) MarkBounced(_ context.Context, _ domain.Message) error {
	return nil
}

func (m *mockMessageWriteRepository) MarkComplained(_ context.Context, _ domain.Message) error {
	return nil
}

func (m *mockMessageWriteRepository) MarkDelayed(_ context.Context, _ domain.Message) error {
	return nil
}

func (m *mockMessageWriteRepository) MarkFailed(_ context.Context, _ domain.Message) error {
	return nil
}

type mockOutboxWriter struct {
	saveFunc func(ctx context.Context, event ports.OutboxEvent) error
}

func (m *mockOutboxWriter) Save(ctx context.Context, event ports.OutboxEvent) error {
	return m.saveFunc(ctx, event)
}

type mockTxManager struct {
	runInTransactionFunc func(ctx context.Context, fn func(ctx context.Context) error) error
}

func (m *mockTxManager) RunInTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return m.runInTransactionFunc(ctx, fn)
}

func validCampaignScheduledPayload(t *testing.T, scheduledAt string) []byte {
	t.Helper()
	payload := contracts.CampaignScheduledPayload{
		CampaignID:        "cmp_1",
		WorkspaceID:       "ws_1",
		TemplateID:        "tpl_1",
		TemplateVersionID: "tv_1",
		SenderDomainID:    "sd_1",
		MessageType:       "marketing",
		ScheduledAt:       scheduledAt,
		PlannedRecipients: 10,
	}
	envelope, err := events.NewEnvelope(events.NewEnvelopeOptions{
		EventID:       "evt_1",
		EventType:     contracts.EventCampaignScheduledV1,
		EventVersion:  1,
		AggregateType: "campaign",
		AggregateID:   "cmp_1",
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

func TestHandleCampaignScheduled_ValidEvent(t *testing.T) {
	campaignReader := &mockCampaignCandidateReader{
		countCandidatesFunc: func(_ context.Context, _, _ string) (int64, error) {
			return 1, nil
		},
		listCandidatesFunc: func(_ context.Context, _, _ string, _ int, _ string) ([]ports.CampaignCandidate, string, error) {
			return []ports.CampaignCandidate{
				{ID: "candidate_1", WorkspaceID: "ws_1", CampaignID: "cmp_1", ContactID: "contact_1", EmailNormalized: "test@example.com"},
			}, "", nil
		},
	}
	msgWrite := &mockMessageWriteRepository{
		createManyFunc: func(_ context.Context, msgs []domain.Message) ([]string, error) {
			ids := make([]string, len(msgs))
			for i, m := range msgs {
				ids[i] = m.ID
			}
			return ids, nil
		},
	}
	outbox := &mockOutboxWriter{
		saveFunc: func(_ context.Context, _ ports.OutboxEvent) error {
			return nil
		},
	}
	txManager := &mockTxManager{
		runInTransactionFunc: func(_ context.Context, fn func(ctx context.Context) error) error {
			return fn(context.Background())
		},
	}

	svc := NewService(Options{
		CampaignReader: campaignReader,
		MessagesWrite:  msgWrite,
		OutboxWriter:   outbox,
		TxManager:      txManager,
	})

	payload := validCampaignScheduledPayload(t, time.Now().UTC().Format(time.RFC3339))
	err := svc.HandleCampaignScheduled(context.Background(), HandleCampaignScheduledInput{
		EventID:    "evt_1",
		RawPayload: payload,
		Now:        time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleCampaignScheduled_DuplicateEvent(t *testing.T) {
	campaignReader := &mockCampaignCandidateReader{
		countCandidatesFunc: func(_ context.Context, _, _ string) (int64, error) {
			return 1, nil
		},
		listCandidatesFunc: func(_ context.Context, _, _ string, _ int, _ string) ([]ports.CampaignCandidate, string, error) {
			return []ports.CampaignCandidate{
				{ID: "candidate_1", WorkspaceID: "ws_1", CampaignID: "cmp_1", ContactID: "contact_1", EmailNormalized: "test@example.com"},
			}, "", nil
		},
	}
	msgWrite := &mockMessageWriteRepository{
		createManyFunc: func(_ context.Context, msgs []domain.Message) ([]string, error) {
			return nil, nil
		},
	}
	outbox := &mockOutboxWriter{
		saveFunc: func(_ context.Context, _ ports.OutboxEvent) error {
			return nil
		},
	}
	txManager := &mockTxManager{
		runInTransactionFunc: func(_ context.Context, fn func(ctx context.Context) error) error {
			return fn(context.Background())
		},
	}

	svc := NewService(Options{
		CampaignReader: campaignReader,
		MessagesWrite:  msgWrite,
		OutboxWriter:   outbox,
		TxManager:      txManager,
	})

	payload := validCampaignScheduledPayload(t, time.Now().UTC().Format(time.RFC3339))
	err := svc.HandleCampaignScheduled(context.Background(), HandleCampaignScheduledInput{
		EventID:    "evt_1",
		RawPayload: payload,
		Now:        time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestHandleCampaignScheduled_InvalidPayload(t *testing.T) {
	svc := NewService(Options{})

	err := svc.HandleCampaignScheduled(context.Background(), HandleCampaignScheduledInput{
		EventID:    "evt_1",
		RawPayload: []byte(`garbage`),
		Now:        time.Now().UTC(),
	})
	var nonRetryable *NonRetryableError
	if !errors.As(err, &nonRetryable) {
		t.Fatalf("expected NonRetryableError, got %T: %v", err, err)
	}
}

func TestHandleCampaignScheduled_NoCandidates(t *testing.T) {
	campaignReader := &mockCampaignCandidateReader{
		countCandidatesFunc: func(_ context.Context, _, _ string) (int64, error) {
			return 0, nil
		},
	}

	svc := NewService(Options{
		CampaignReader: campaignReader,
	})

	payload := validCampaignScheduledPayload(t, time.Now().UTC().Format(time.RFC3339))
	err := svc.HandleCampaignScheduled(context.Background(), HandleCampaignScheduledInput{
		EventID:    "evt_1",
		RawPayload: payload,
		Now:        time.Now().UTC(),
	})
	if !errors.Is(err, domain.ErrCampaignCandidatesNotFound) {
		t.Fatalf("expected ErrCampaignCandidatesNotFound, got %v", err)
	}
}

func TestHandleCampaignScheduled_EmptyEventID(t *testing.T) {
	svc := NewService(Options{})

	payload := validCampaignScheduledPayload(t, time.Now().UTC().Format(time.RFC3339))
	err := svc.HandleCampaignScheduled(context.Background(), HandleCampaignScheduledInput{
		EventID:    "",
		RawPayload: payload,
		Now:        time.Now().UTC(),
	})
	var nonRetryable *NonRetryableError
	if !errors.As(err, &nonRetryable) {
		t.Fatalf("expected NonRetryableError, got %T: %v", err, err)
	}
}
