package app

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/ports"
)

func TestProviderEventClassifier_ClassifyEventType(t *testing.T) {
	tests := []struct {
		eventType    string
		expectStatus string
		expectOK     bool
	}{
		{"delivered", domain.MessageStatusDelivered, true},
		{"bounced", domain.MessageStatusBounced, true},
		{"complained", domain.MessageStatusComplained, true},
		{"accepted", "", false},
		{"opened", "", false},
		{"clicked", "", false},
		{"unsubscribed", "", false},
		{"rendering_failed", "", false},
		{"delayed", domain.MessageStatusDelayed, true},
		{"rejected", domain.MessageStatusFailed, true},
		{"unknown", "", false},
		{"", "", false},
	}

	for _, tt := range tests {
		status, ok := domain.ClassifyProviderEvent(tt.eventType)
		if ok != tt.expectOK {
			t.Errorf("ClassifyProviderEvent(%q) ok = %v, want %v", tt.eventType, ok, tt.expectOK)
		}
		if status != tt.expectStatus {
			t.Errorf("ClassifyProviderEvent(%q) status = %q, want %q", tt.eventType, status, tt.expectStatus)
		}
	}
}

func TestProviderEventClassifier_CanTransition(t *testing.T) {
	tests := []struct {
		current string
		target  string
		allowed bool
	}{
		{domain.MessageStatusQueued, domain.MessageStatusAccepted, true},
		{domain.MessageStatusQueued, domain.MessageStatusFailed, true},
		{domain.MessageStatusQueued, domain.MessageStatusDelivered, false},
		{domain.MessageStatusQueued, domain.MessageStatusComplained, false},
		{domain.MessageStatusProcessing, domain.MessageStatusAccepted, false},
		{domain.MessageStatusProcessing, domain.MessageStatusFailed, true},
		{domain.MessageStatusProcessing, domain.MessageStatusDelivered, true},
		{domain.MessageStatusProcessing, domain.MessageStatusBounced, true},
		{domain.MessageStatusProcessing, domain.MessageStatusComplained, true},
		{domain.MessageStatusAccepted, domain.MessageStatusDelivered, true},
		{domain.MessageStatusAccepted, domain.MessageStatusBounced, true},
		{domain.MessageStatusAccepted, domain.MessageStatusComplained, true},
		{domain.MessageStatusAccepted, domain.MessageStatusFailed, true},
		{domain.MessageStatusDelayed, domain.MessageStatusDelivered, true},
		{domain.MessageStatusDelayed, domain.MessageStatusBounced, true},
		{domain.MessageStatusDelayed, domain.MessageStatusFailed, true},
		{domain.MessageStatusDelayed, domain.MessageStatusAccepted, false},

		{domain.MessageStatusBounced, domain.MessageStatusDelivered, false},
		{domain.MessageStatusBounced, domain.MessageStatusComplained, false},
		{domain.MessageStatusBounced, domain.MessageStatusAccepted, false},
		{domain.MessageStatusFailed, domain.MessageStatusDelivered, false},
		{domain.MessageStatusCancelled, domain.MessageStatusDelivered, false},
		{domain.MessageStatusDLQ, domain.MessageStatusDelivered, false},
		{domain.MessageStatusComplained, domain.MessageStatusDelivered, false},
		{domain.MessageStatusComplained, domain.MessageStatusBounced, false},
		{domain.MessageStatusDelivered, domain.MessageStatusAccepted, false},
		{domain.MessageStatusDelivered, domain.MessageStatusDelayed, false},
		{domain.MessageStatusDelivered, domain.MessageStatusBounced, false},
		{domain.MessageStatusDelivered, domain.MessageStatusComplained, true},
	}

	for _, tt := range tests {
		got := domain.CanTransitionToStatus(tt.current, tt.target)
		if got != tt.allowed {
			t.Errorf("CanTransitionToStatus(%q -> %q) = %v, want %v", tt.current, tt.target, got, tt.allowed)
		}
	}
}

func TestHandleProviderEvent_Delivered(t *testing.T) {
	now := time.Now().UTC()
	occurredAt := now.Add(-time.Minute)

	msg := &domain.Message{
		ID:                       "msg_1",
		WorkspaceID:              "ws_1",
		Status:                   domain.MessageStatusAccepted,
		RecipientEmailNormalized: "test@example.com",
		RecipientSnapshot: domain.RecipientSnapshot{
			Email:           "test@example.com",
			EmailNormalized: "test@example.com",
		},
		AcceptedAt: &occurredAt,
	}

	var capturedUpdate *domain.Message
	outboxEvents := 0

	svc := NewService(Options{
		MessagesRead: &mockMessageReadRepo{
			findByID: func(ctx context.Context, workspaceID, messageID string) (*domain.Message, error) {
				return msg, nil
			},
		},
		MessagesWrite: &mockMessageWriteRepo{
			update: func(ctx context.Context, m domain.Message) error {
				capturedUpdate = &m
				return nil
			},
		},
		OutboxWriter: &mockOutboxWriter{
			saveFunc: func(ctx context.Context, event ports.OutboxEvent) error {
				outboxEvents++
				return nil
			},
		},
		TxManager: &mockTxManager{
			withinTxFunc: func(ctx context.Context, fn func(ctx context.Context) error) error {
				return fn(ctx)
			},
		},
		IDGen:  func() (string, error) { return "evt_out_1", nil },
		Logger: testLogger(),
	})

	result, err := svc.HandleProviderEvent(context.Background(), HandleProviderEventInput{
		EventID:           "evt_1",
		NormalizedEventID: "norm_1",
		RawEventID:        "raw_1",
		WorkspaceID:       "ws_1",
		MessageID:         "msg_1",
		Provider:          "fake",
		ProviderEventID:   "prov_evt_1",
		ProviderMessageID: "prov_msg_1",
		EventType:         "delivered",
		OccurredAt:        occurredAt,
		ReceivedAt:        now,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Handled {
		t.Fatal("expected handled")
	}
	if result.NewStatus != domain.MessageStatusDelivered {
		t.Errorf("expected status delivered, got %s", result.NewStatus)
	}
	if outboxEvents != 1 {
		t.Errorf("expected 1 outbox event, got %d", outboxEvents)
	}
	if capturedUpdate == nil || capturedUpdate.Status != domain.MessageStatusDelivered {
		t.Error("expected message to be updated to delivered")
	}
}

func TestHandleProviderEvent_Bounced(t *testing.T) {
	now := time.Now().UTC()
	occurredAt := now.Add(-time.Minute)

	msg := &domain.Message{
		ID:                       "msg_1",
		WorkspaceID:              "ws_1",
		Status:                   domain.MessageStatusAccepted,
		RecipientEmailNormalized: "test@example.com",
		RecipientSnapshot: domain.RecipientSnapshot{
			Email:           "test@example.com",
			EmailNormalized: "test@example.com",
		},
	}

	var capturedUpdate *domain.Message
	var capturedSuppression *SuppressFromSignalInput
	outboxEvents := 0

	svc := NewService(Options{
		MessagesRead: &mockMessageReadRepo{
			findByID: func(ctx context.Context, workspaceID, messageID string) (*domain.Message, error) {
				return msg, nil
			},
		},
		MessagesWrite: &mockMessageWriteRepo{
			update: func(ctx context.Context, m domain.Message) error {
				capturedUpdate = &m
				return nil
			},
		},
		RecipientSuppressor: &mockRecipientSuppressor{
			suppressFromSignal: func(ctx context.Context, input SuppressFromSignalInput) (*SuppressFromSignalResult, error) {
				capturedSuppression = &input
				return &SuppressFromSignalResult{EntryID: "sup_1", Created: true}, nil
			},
		},
		OutboxWriter: &mockOutboxWriter{
			saveFunc: func(ctx context.Context, event ports.OutboxEvent) error {
				outboxEvents++
				return nil
			},
		},
		TxManager: &mockTxManager{
			withinTxFunc: func(ctx context.Context, fn func(ctx context.Context) error) error {
				return fn(ctx)
			},
		},
		IDGen:  func() (string, error) { return "evt_out_1", nil },
		Logger: testLogger(),
	})

	result, err := svc.HandleProviderEvent(context.Background(), HandleProviderEventInput{
		EventID:           "evt_1",
		NormalizedEventID: "norm_1",
		RawEventID:        "raw_1",
		WorkspaceID:       "ws_1",
		MessageID:         "msg_1",
		Provider:          "fake",
		ProviderEventID:   "prov_evt_1",
		ProviderMessageID: "prov_msg_1",
		EventType:         "bounced",
		OccurredAt:        occurredAt,
		ReceivedAt:        now,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Handled {
		t.Fatal("expected handled")
	}
	if result.NewStatus != domain.MessageStatusBounced {
		t.Errorf("expected status bounced, got %s", result.NewStatus)
	}
	if !result.SuppressionCreated {
		t.Fatal("expected suppression created")
	}
	// 2 outbox events: one for bounced, one for suppression
	if outboxEvents != 2 {
		t.Errorf("expected 2 outbox events (bounced + suppression), got %d", outboxEvents)
	}
	if capturedUpdate == nil || capturedUpdate.Status != domain.MessageStatusBounced {
		t.Error("expected message to be updated to bounced")
	}
	if capturedSuppression == nil || capturedSuppression.Reason != "bounce" {
		t.Errorf("expected suppression reason bounce, got %v", capturedSuppression)
	}
}

func TestHandleProviderEvent_Complained(t *testing.T) {
	now := time.Now().UTC()
	occurredAt := now.Add(-time.Minute)

	msg := &domain.Message{
		ID:                       "msg_1",
		WorkspaceID:              "ws_1",
		Status:                   domain.MessageStatusAccepted,
		RecipientEmailNormalized: "test@example.com",
		RecipientSnapshot: domain.RecipientSnapshot{
			Email:           "test@example.com",
			EmailNormalized: "test@example.com",
		},
	}

	outboxEvents := 0

	svc := NewService(Options{
		MessagesRead: &mockMessageReadRepo{
			findByID: func(ctx context.Context, workspaceID, messageID string) (*domain.Message, error) {
				return msg, nil
			},
		},
		MessagesWrite: &mockMessageWriteRepo{
			update: func(ctx context.Context, m domain.Message) error {
				return nil
			},
		},
		RecipientSuppressor: &mockRecipientSuppressor{
			suppressFromSignal: func(ctx context.Context, input SuppressFromSignalInput) (*SuppressFromSignalResult, error) {
				return &SuppressFromSignalResult{EntryID: "sup_1", Created: true}, nil
			},
		},
		OutboxWriter: &mockOutboxWriter{
			saveFunc: func(ctx context.Context, event ports.OutboxEvent) error {
				outboxEvents++
				return nil
			},
		},
		TxManager: &mockTxManager{
			withinTxFunc: func(ctx context.Context, fn func(ctx context.Context) error) error {
				return fn(ctx)
			},
		},
		IDGen:  func() (string, error) { return "evt_out_1", nil },
		Logger: testLogger(),
	})

	result, err := svc.HandleProviderEvent(context.Background(), HandleProviderEventInput{
		EventID:           "evt_1",
		NormalizedEventID: "norm_1",
		RawEventID:        "raw_1",
		WorkspaceID:       "ws_1",
		MessageID:         "msg_1",
		Provider:          "fake",
		ProviderEventID:   "prov_evt_1",
		ProviderMessageID: "prov_msg_1",
		EventType:         "complained",
		OccurredAt:        occurredAt,
		ReceivedAt:        now,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Handled {
		t.Fatal("expected handled")
	}
	if result.NewStatus != domain.MessageStatusComplained {
		t.Errorf("expected status complained, got %s", result.NewStatus)
	}
	if !result.SuppressionCreated {
		t.Fatal("expected suppression created")
	}
	if outboxEvents != 2 {
		t.Errorf("expected 2 outbox events (complained + suppression), got %d", outboxEvents)
	}
}

func TestHandleProviderEvent_ComplaintOnAccepted(t *testing.T) {
	now := time.Now().UTC()
	occurredAt := now.Add(-time.Minute)

	msg := &domain.Message{
		ID:                       "msg_1",
		WorkspaceID:              "ws_1",
		Status:                   domain.MessageStatusAccepted,
		RecipientEmailNormalized: "test@example.com",
	}

	svc := NewService(Options{
		MessagesRead: &mockMessageReadRepo{
			findByID: func(ctx context.Context, workspaceID, messageID string) (*domain.Message, error) {
				return msg, nil
			},
		},
		MessagesWrite: &mockMessageWriteRepo{
			update: func(ctx context.Context, m domain.Message) error {
				return nil
			},
		},
		RecipientSuppressor: &mockRecipientSuppressor{
			suppressFromSignal: func(ctx context.Context, input SuppressFromSignalInput) (*SuppressFromSignalResult, error) {
				return &SuppressFromSignalResult{EntryID: "sup_1", Created: true}, nil
			},
		},
		OutboxWriter: &mockOutboxWriter{
			saveFunc: func(ctx context.Context, event ports.OutboxEvent) error {
				return nil
			},
		},
		TxManager: &mockTxManager{
			withinTxFunc: func(ctx context.Context, fn func(ctx context.Context) error) error {
				return fn(ctx)
			},
		},
		IDGen:  func() (string, error) { return "evt_out_1", nil },
		Logger: testLogger(),
	})

	result, err := svc.HandleProviderEvent(context.Background(), HandleProviderEventInput{
		EventID:           "evt_1",
		NormalizedEventID: "norm_1",
		RawEventID:        "raw_1",
		WorkspaceID:       "ws_1",
		MessageID:         "msg_1",
		Provider:          "fake",
		ProviderEventID:   "prov_evt_1",
		ProviderMessageID: "prov_msg_1",
		EventType:         "complained",
		OccurredAt:        occurredAt,
		ReceivedAt:        now,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Handled {
		t.Fatal("expected handled")
	}
	if result.NewStatus != domain.MessageStatusComplained {
		t.Errorf("expected status complained, got %s", result.NewStatus)
	}
}

func TestHandleProviderEvent_BounceOnQueuedIsNoop(t *testing.T) {
	now := time.Now().UTC()
	occurredAt := now.Add(-time.Minute)

	msg := &domain.Message{
		ID:                       "msg_1",
		WorkspaceID:              "ws_1",
		Status:                   domain.MessageStatusQueued,
		RecipientEmailNormalized: "test@example.com",
	}

	updateCalled := false

	svc := NewService(Options{
		MessagesRead: &mockMessageReadRepo{
			findByID: func(ctx context.Context, workspaceID, messageID string) (*domain.Message, error) {
				return msg, nil
			},
		},
		MessagesWrite: &mockMessageWriteRepo{
			update: func(ctx context.Context, m domain.Message) error {
				updateCalled = true
				return nil
			},
		},
		OutboxWriter: &mockOutboxWriter{
			saveFunc: func(ctx context.Context, event ports.OutboxEvent) error {
				return nil
			},
		},
		TxManager: &mockTxManager{
			withinTxFunc: func(ctx context.Context, fn func(ctx context.Context) error) error {
				return fn(ctx)
			},
		},
		IDGen:  func() (string, error) { return "evt_out_1", nil },
		Logger: testLogger(),
	})

	result, err := svc.HandleProviderEvent(context.Background(), HandleProviderEventInput{
		EventID:           "evt_1",
		NormalizedEventID: "norm_1",
		RawEventID:        "raw_1",
		WorkspaceID:       "ws_1",
		MessageID:         "msg_1",
		Provider:          "fake",
		ProviderEventID:   "prov_evt_1",
		ProviderMessageID: "prov_msg_1",
		EventType:         "bounced",
		OccurredAt:        occurredAt,
		ReceivedAt:        now,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Handled {
		t.Fatal("expected handled")
	}
	if result.NewStatus != domain.MessageStatusBounced {
		t.Errorf("expected status bounced, got %s", result.NewStatus)
	}
	if !updateCalled {
		t.Fatal("expected update for valid transition from queued to bounced")
	}
}

func TestHandleProviderEvent_IgnoredEventType(t *testing.T) {
	now := time.Now().UTC()
	occurredAt := now.Add(-time.Minute)

	svc := NewService(Options{
		Logger: testLogger(),
	})

	result, err := svc.HandleProviderEvent(context.Background(), HandleProviderEventInput{
		EventID:           "evt_1",
		NormalizedEventID: "norm_1",
		RawEventID:        "raw_1",
		WorkspaceID:       "ws_1",
		MessageID:         "msg_1",
		Provider:          "fake",
		ProviderEventID:   "prov_evt_1",
		ProviderMessageID: "prov_msg_1",
		EventType:         "opened",
		OccurredAt:        occurredAt,
		ReceivedAt:        now,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Ignored {
		t.Fatal("expected ignored")
	}
}

func TestHandleProviderEvent_MissingMessageIsIgnored(t *testing.T) {
	now := time.Now().UTC()
	occurredAt := now.Add(-time.Minute)

	svc := NewService(Options{
		MessagesRead: &mockMessageReadRepo{
			findByID: func(ctx context.Context, workspaceID, messageID string) (*domain.Message, error) {
				return nil, domain.ErrMessageNotFound
			},
		},
		Logger: testLogger(),
	})

	result, err := svc.HandleProviderEvent(context.Background(), HandleProviderEventInput{
		EventID:           "evt_1",
		NormalizedEventID: "norm_1",
		WorkspaceID:       "ws_1",
		MessageID:         "msg_1",
		Provider:          "fake",
		EventType:         "delivered",
		OccurredAt:        occurredAt,
		ReceivedAt:        now,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Ignored {
		t.Fatal("expected ignored when message not found")
	}
}

func TestHandleProviderEvent_DuplicateTerminalIsNoop(t *testing.T) {
	now := time.Now().UTC()
	occurredAt := now.Add(-time.Minute)

	msg := &domain.Message{
		ID:                       "msg_1",
		WorkspaceID:              "ws_1",
		Status:                   domain.MessageStatusDelivered,
		RecipientEmailNormalized: "test@example.com",
		DeliveredAt:              &occurredAt,
	}

	updateCalled := false

	svc := NewService(Options{
		MessagesRead: &mockMessageReadRepo{
			findByID: func(ctx context.Context, workspaceID, messageID string) (*domain.Message, error) {
				return msg, nil
			},
		},
		MessagesWrite: &mockMessageWriteRepo{
			update: func(ctx context.Context, m domain.Message) error {
				updateCalled = true
				return nil
			},
		},
		Logger: testLogger(),
	})

	result, err := svc.HandleProviderEvent(context.Background(), HandleProviderEventInput{
		EventID:           "evt_1",
		NormalizedEventID: "norm_1",
		WorkspaceID:       "ws_1",
		MessageID:         "msg_1",
		Provider:          "fake",
		ProviderMessageID: "prov_msg_1",
		EventType:         "delivered",
		OccurredAt:        occurredAt,
		ReceivedAt:        now,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Handled != true {
		t.Fatal("expected handled (no-op)")
	}
	if updateCalled {
		t.Fatal("expected no update for duplicate terminal event")
	}
}

func TestHandleProviderEvent_DeliveredDoesNotOverrideComplained(t *testing.T) {
	now := time.Now().UTC()
	occurredAt := now.Add(-time.Minute)

	msg := &domain.Message{
		ID:                       "msg_1",
		WorkspaceID:              "ws_1",
		Status:                   domain.MessageStatusComplained,
		RecipientEmailNormalized: "test@example.com",
		ComplainedAt:             &occurredAt,
	}

	updateCalled := false

	svc := NewService(Options{
		MessagesRead: &mockMessageReadRepo{
			findByID: func(ctx context.Context, workspaceID, messageID string) (*domain.Message, error) {
				return msg, nil
			},
		},
		MessagesWrite: &mockMessageWriteRepo{
			update: func(ctx context.Context, m domain.Message) error {
				updateCalled = true
				return nil
			},
		},
		Logger: testLogger(),
	})

	result, err := svc.HandleProviderEvent(context.Background(), HandleProviderEventInput{
		EventID:           "evt_1",
		NormalizedEventID: "norm_1",
		WorkspaceID:       "ws_1",
		MessageID:         "msg_1",
		Provider:          "fake",
		EventType:         "delivered",
		OccurredAt:        occurredAt,
		ReceivedAt:        now,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.NewStatus != domain.MessageStatusComplained {
		t.Errorf("expected status to remain complained, got %s", result.NewStatus)
	}
	if updateCalled {
		t.Fatal("expected no update when delivered cannot override complained")
	}
}

func TestHandleProviderEvent_BouncedDoesNotOverrideComplained(t *testing.T) {
	now := time.Now().UTC()
	occurredAt := now.Add(-time.Minute)

	msg := &domain.Message{
		ID:                       "msg_1",
		WorkspaceID:              "ws_1",
		Status:                   domain.MessageStatusComplained,
		RecipientEmailNormalized: "test@example.com",
		ComplainedAt:             &occurredAt,
	}

	updateCalled := false

	svc := NewService(Options{
		MessagesRead: &mockMessageReadRepo{
			findByID: func(ctx context.Context, workspaceID, messageID string) (*domain.Message, error) {
				return msg, nil
			},
		},
		MessagesWrite: &mockMessageWriteRepo{
			update: func(ctx context.Context, m domain.Message) error {
				updateCalled = true
				return nil
			},
		},
		Logger: testLogger(),
	})

	result, err := svc.HandleProviderEvent(context.Background(), HandleProviderEventInput{
		EventID:           "evt_1",
		NormalizedEventID: "norm_1",
		WorkspaceID:       "ws_1",
		MessageID:         "msg_1",
		Provider:          "fake",
		EventType:         "bounced",
		OccurredAt:        occurredAt,
		ReceivedAt:        now,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.NewStatus != domain.MessageStatusComplained {
		t.Errorf("expected status to remain complained, got %s", result.NewStatus)
	}
	if updateCalled {
		t.Fatal("expected no update when bounced cannot override complained")
	}
}

func TestHandleProviderEvent_AcceptedCanTransitionToBounced(t *testing.T) {
	now := time.Now().UTC()
	occurredAt := now.Add(-time.Minute)

	msg := &domain.Message{
		ID:                       "msg_1",
		WorkspaceID:              "ws_1",
		Status:                   domain.MessageStatusAccepted,
		RecipientEmailNormalized: "test@example.com",
		RecipientSnapshot: domain.RecipientSnapshot{
			Email:           "test@example.com",
			EmailNormalized: "test@example.com",
		},
	}

	svc := NewService(Options{
		MessagesRead: &mockMessageReadRepo{
			findByID: func(ctx context.Context, workspaceID, messageID string) (*domain.Message, error) {
				return msg, nil
			},
		},
		MessagesWrite: &mockMessageWriteRepo{
			update: func(ctx context.Context, m domain.Message) error {
				return nil
			},
		},
		RecipientSuppressor: &mockRecipientSuppressor{
			suppressFromSignal: func(ctx context.Context, input SuppressFromSignalInput) (*SuppressFromSignalResult, error) {
				return &SuppressFromSignalResult{EntryID: "sup_1", Created: true}, nil
			},
		},
		OutboxWriter: &mockOutboxWriter{
			saveFunc: func(ctx context.Context, event ports.OutboxEvent) error {
				return nil
			},
		},
		TxManager: &mockTxManager{
			withinTxFunc: func(ctx context.Context, fn func(ctx context.Context) error) error {
				return fn(ctx)
			},
		},
		IDGen:  func() (string, error) { return "evt_out_1", nil },
		Logger: testLogger(),
	})

	result, err := svc.HandleProviderEvent(context.Background(), HandleProviderEventInput{
		EventID:           "evt_1",
		NormalizedEventID: "norm_1",
		WorkspaceID:       "ws_1",
		MessageID:         "msg_1",
		Provider:          "fake",
		ProviderEventID:   "prov_evt_1",
		ProviderMessageID: "prov_msg_1",
		EventType:         "bounced",
		OccurredAt:        occurredAt,
		ReceivedAt:        now,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.NewStatus != domain.MessageStatusBounced {
		t.Errorf("expected status bounced, got %s", result.NewStatus)
	}
}

func TestHandleProviderEvent_InvalidInput(t *testing.T) {
	svc := NewService(Options{
		Logger: testLogger(),
	})

	_, err := svc.HandleProviderEvent(context.Background(), HandleProviderEventInput{
		EventID: "",
	})
	if err == nil {
		t.Fatal("expected error for empty event_id")
	}
	var nonRetryable *NonRetryableError
	if !errors.As(err, &nonRetryable) {
		t.Fatalf("expected NonRetryableError, got %T: %v", err, err)
	}
}

func TestHandleProviderEvent_ProviderMessageIDLookup(t *testing.T) {
	now := time.Now().UTC()
	occurredAt := now.Add(-time.Minute)

	msg := &domain.Message{
		ID:                       "msg_1",
		WorkspaceID:              "ws_1",
		Status:                   domain.MessageStatusAccepted,
		RecipientEmailNormalized: "test@example.com",
		RecipientSnapshot: domain.RecipientSnapshot{
			Email:           "test@example.com",
			EmailNormalized: "test@example.com",
		},
		Provider:          "fake",
		ProviderMessageID: "prov_msg_1",
	}

	svc := NewService(Options{
		MessagesRead: &mockMessageReadRepo{
			findByID: func(ctx context.Context, workspaceID, messageID string) (*domain.Message, error) {
				return msg, nil
			},
			findByProviderMessageID: func(ctx context.Context, provider, providerMessageID string) (*domain.Message, error) {
				return msg, nil
			},
		},
		MessagesWrite: &mockMessageWriteRepo{
			update: func(ctx context.Context, m domain.Message) error {
				return nil
			},
		},
		OutboxWriter: &mockOutboxWriter{
			saveFunc: func(ctx context.Context, event ports.OutboxEvent) error {
				return nil
			},
		},
		TxManager: &mockTxManager{
			withinTxFunc: func(ctx context.Context, fn func(ctx context.Context) error) error {
				return fn(ctx)
			},
		},
		IDGen:  func() (string, error) { return "evt_out_1", nil },
		Logger: testLogger(),
	})

	result, err := svc.HandleProviderEvent(context.Background(), HandleProviderEventInput{
		EventID:           "evt_1",
		NormalizedEventID: "norm_1",
		Provider:          "fake",
		ProviderMessageID: "prov_msg_1",
		EventType:         "delivered",
		OccurredAt:        occurredAt,
		ReceivedAt:        now,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Handled {
		t.Fatal("expected handled")
	}
	if result.NewStatus != domain.MessageStatusDelivered {
		t.Errorf("expected status delivered, got %s", result.NewStatus)
	}
}

func TestHandleProviderEvent_OutboxFailureRollsBack(t *testing.T) {
	now := time.Now().UTC()
	occurredAt := now.Add(-time.Minute)

	msg := &domain.Message{
		ID:                       "msg_1",
		WorkspaceID:              "ws_1",
		Status:                   domain.MessageStatusAccepted,
		RecipientEmailNormalized: "test@example.com",
	}

	updateCalled := false

	svc := NewService(Options{
		MessagesRead: &mockMessageReadRepo{
			findByID: func(ctx context.Context, workspaceID, messageID string) (*domain.Message, error) {
				return msg, nil
			},
		},
		MessagesWrite: &mockMessageWriteRepo{
			update: func(ctx context.Context, m domain.Message) error {
				updateCalled = true
				return nil
			},
		},
		OutboxWriter: &mockOutboxWriter{
			saveFunc: func(ctx context.Context, event ports.OutboxEvent) error {
				return errors.New("outbox save failed")
			},
		},
		TxManager: &mockTxManager{
			withinTxFunc: func(ctx context.Context, fn func(ctx context.Context) error) error {
				return fn(ctx)
			},
		},
		IDGen:  func() (string, error) { return "evt_out_1", nil },
		Logger: testLogger(),
	})

	_, err := svc.HandleProviderEvent(context.Background(), HandleProviderEventInput{
		EventID:           "evt_1",
		NormalizedEventID: "norm_1",
		WorkspaceID:       "ws_1",
		MessageID:         "msg_1",
		Provider:          "fake",
		ProviderMessageID: "prov_msg_1",
		EventType:         "delivered",
		OccurredAt:        occurredAt,
		ReceivedAt:        now,
	})
	if err == nil {
		t.Fatal("expected error from outbox failure")
	}
	if !updateCalled {
		t.Fatal("expected update to be called before outbox failure")
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
