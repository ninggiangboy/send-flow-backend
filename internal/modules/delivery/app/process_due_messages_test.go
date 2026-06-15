package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/contracts"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/ports"
)

func dueMessage() domain.Message {
	return domain.Message{
		ID:                       "msg_1",
		WorkspaceID:              "ws_1",
		RecipientEmailNormalized: "test@example.com",
		RecipientSnapshot: domain.RecipientSnapshot{
			Email:     "test@example.com",
			FirstName: "John",
			LastName:  "Doe",
		},
		TemplateID:        "tmpl_1",
		TemplateVersionID: "tv_1",
		SenderDomainID:    "sd_1",
		MessageType:       "marketing",
		SourceType:        "campaign",
		Status:            domain.MessageStatusQueued,
	}
}

func TestProcessDueMessages_Accepted(t *testing.T) {
	opts := newTestOpts()

	msg := dueMessage()
	write := opts.MessagesWrite.(*mockMessageWriteRepo)
	write.claimDueMessages = func(ctx context.Context, query ports.DueMessageQuery, now time.Time) ([]domain.Message, error) {
		return []domain.Message{msg}, nil
	}
	write.markAccepted = func(ctx context.Context, message domain.Message) error {
		if message.Provider != "test" || message.ProviderMessageID != "prov_msg_1" {
			t.Errorf("unexpected provider values: %s / %s", message.Provider, message.ProviderMessageID)
		}
		return nil
	}

	attemptRead := opts.AttemptsRead.(*mockAttemptReadRepo)
	attemptRead.nextAttemptNumber = func(ctx context.Context, workspaceID, messageID string) (int, error) {
		return 1, nil
	}
	attemptWrite := opts.AttemptsWrite.(*mockAttemptWriteRepo)
	attemptWrite.create = func(ctx context.Context, attempt domain.DeliveryAttempt) error {
		return nil
	}
	attemptWrite.update = func(ctx context.Context, attempt domain.DeliveryAttempt) error {
		if attempt.Status != domain.AttemptStatusAccepted {
			t.Errorf("expected attempt status accepted, got %s", attempt.Status)
		}
		return nil
	}

	var outboxSaved bool
	outbox := opts.OutboxWriter.(*mockOutboxWriter)
	outbox.saveFunc = func(ctx context.Context, event ports.OutboxEvent) error {
		if event.EventType == contracts.EventDeliveryMessageAcceptedV1 {
			outboxSaved = true
		}
		return nil
	}

	svc := NewService(opts)
	result, err := svc.ProcessDueMessages(context.Background(), ProcessDueMessagesInput{
		WorkspaceID: "ws_1",
		MessageType: "marketing",
		Limit:       10,
		Now:         time.Now(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.SelectedCount != 1 {
		t.Errorf("expected 1 selected, got %d", result.SelectedCount)
	}
	if !outboxSaved {
		t.Error("expected accepted outbox event")
	}
}

func TestProcessDueMessages_SenderNotReady(t *testing.T) {
	opts := newTestOpts()

	msg := dueMessage()
	write := opts.MessagesWrite.(*mockMessageWriteRepo)
	write.claimDueMessages = func(ctx context.Context, query ports.DueMessageQuery, now time.Time) ([]domain.Message, error) {
		return []domain.Message{msg}, nil
	}
	write.markFailed = func(ctx context.Context, message domain.Message) error {
		if message.LastErrorClass != "sender_not_ready" {
			t.Errorf("expected error class sender_not_ready, got %s", message.LastErrorClass)
		}
		return nil
	}

	sender := opts.SenderChecker.(*mockSenderChecker)
	sender.getSenderReadiness = func(ctx context.Context, workspaceID, senderDomainID string) (*ports.SenderReadiness, error) {
		return &ports.SenderReadiness{Ready: false}, nil
	}

	svc := NewService(opts)
	_, err := svc.ProcessDueMessages(context.Background(), ProcessDueMessagesInput{
		WorkspaceID: "ws_1",
		MessageType: "marketing",
		Limit:       10,
		Now:         time.Now(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestProcessDueMessages_RecipientSuppressed(t *testing.T) {
	opts := newTestOpts()

	msg := dueMessage()
	write := opts.MessagesWrite.(*mockMessageWriteRepo)
	write.claimDueMessages = func(ctx context.Context, query ports.DueMessageQuery, now time.Time) ([]domain.Message, error) {
		return []domain.Message{msg}, nil
	}
	write.markFailed = func(ctx context.Context, message domain.Message) error {
		if message.LastErrorClass != "recipient_suppressed" {
			t.Errorf("expected error class recipient_suppressed, got %s", message.LastErrorClass)
		}
		return nil
	}

	sup := opts.SuppressionChecker.(*mockSuppressionChecker)
	sup.checkSuppression = func(ctx context.Context, workspaceID, emailNormalized, scope string) (*ports.SuppressionDecision, error) {
		return &ports.SuppressionDecision{Suppressed: true, Reason: "manual_block", Scope: "workspace"}, nil
	}

	svc := NewService(opts)
	_, err := svc.ProcessDueMessages(context.Background(), ProcessDueMessagesInput{
		WorkspaceID: "ws_1",
		MessageType: "marketing",
		Limit:       10,
		Now:         time.Now(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestProcessDueMessages_RenderFailure(t *testing.T) {
	opts := newTestOpts()

	msg := dueMessage()
	write := opts.MessagesWrite.(*mockMessageWriteRepo)
	write.claimDueMessages = func(ctx context.Context, query ports.DueMessageQuery, now time.Time) ([]domain.Message, error) {
		return []domain.Message{msg}, nil
	}
	write.markFailed = func(ctx context.Context, message domain.Message) error {
		if message.LastErrorClass != "template_render_failed" {
			t.Errorf("expected error class template_render_failed, got %s", message.LastErrorClass)
		}
		return nil
	}

	renderer := opts.ContentRenderer.(*mockContentRenderer)
	renderer.renderForMessage = func(ctx context.Context, workspaceID, templateID, templateVersionID string, data map[string]any) (*ports.RenderedMessage, error) {
		return nil, errors.New("render error")
	}

	svc := NewService(opts)
	_, err := svc.ProcessDueMessages(context.Background(), ProcessDueMessagesInput{
		WorkspaceID: "ws_1",
		MessageType: "marketing",
		Limit:       10,
		Now:         time.Now(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestProcessDueMessages_ProviderPermanentFailure(t *testing.T) {
	opts := newTestOpts()

	msg := dueMessage()
	write := opts.MessagesWrite.(*mockMessageWriteRepo)
	write.claimDueMessages = func(ctx context.Context, query ports.DueMessageQuery, now time.Time) ([]domain.Message, error) {
		return []domain.Message{msg}, nil
	}
	write.markFailed = func(ctx context.Context, message domain.Message) error {
		if message.LastErrorClass != "provider_permanent_failure" {
			t.Errorf("expected error class provider_permanent_failure, got %s", message.LastErrorClass)
		}
		return nil
	}

	attemptRead := opts.AttemptsRead.(*mockAttemptReadRepo)
	attemptRead.nextAttemptNumber = func(ctx context.Context, workspaceID, messageID string) (int, error) {
		return 1, nil
	}
	attemptWrite := opts.AttemptsWrite.(*mockAttemptWriteRepo)
	attemptWrite.create = func(ctx context.Context, attempt domain.DeliveryAttempt) error {
		return nil
	}
	attemptWrite.update = func(ctx context.Context, attempt domain.DeliveryAttempt) error {
		return nil
	}

	provider := opts.EmailProvider.(*mockEmailProvider)
	provider.sendEmail = func(ctx context.Context, request ports.ProviderSendRequest) (*ports.ProviderSendResult, error) {
		return nil, domain.ErrProviderPermanentFailure
	}

	svc := NewService(opts)
	_, err := svc.ProcessDueMessages(context.Background(), ProcessDueMessagesInput{
		WorkspaceID: "ws_1",
		MessageType: "marketing",
		Limit:       10,
		Now:         time.Now(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestProcessDueMessages_ProviderTemporaryFailure(t *testing.T) {
	opts := newTestOpts()

	msg := dueMessage()
	write := opts.MessagesWrite.(*mockMessageWriteRepo)
	write.claimDueMessages = func(ctx context.Context, query ports.DueMessageQuery, now time.Time) ([]domain.Message, error) {
		return []domain.Message{msg}, nil
	}
	write.update = func(ctx context.Context, message domain.Message) error {
		if message.Status != domain.MessageStatusQueued {
			t.Errorf("expected status queued, got %s", message.Status)
		}
		if message.ScheduledAt == nil {
			t.Errorf("expected scheduled_at to be set")
		}
		return nil
	}

	attemptRead := opts.AttemptsRead.(*mockAttemptReadRepo)
	attemptRead.nextAttemptNumber = func(ctx context.Context, workspaceID, messageID string) (int, error) {
		return 1, nil
	}
	attemptWrite := opts.AttemptsWrite.(*mockAttemptWriteRepo)
	attemptWrite.create = func(ctx context.Context, attempt domain.DeliveryAttempt) error {
		return nil
	}
	attemptWrite.update = func(ctx context.Context, attempt domain.DeliveryAttempt) error {
		return nil
	}

	retryWrite := opts.RetryStatesWrite.(*mockRetryStateWriteRepo)
	var retryCreated bool
	retryWrite.create = func(ctx context.Context, state domain.RetryState) error {
		retryCreated = true
		if state.RetryCount != 1 {
			t.Errorf("expected retry count 1, got %d", state.RetryCount)
		}
		if state.Status != domain.RetryStatusScheduled {
			t.Errorf("expected status scheduled, got %s", state.Status)
		}
		return nil
	}

	provider := opts.EmailProvider.(*mockEmailProvider)
	provider.sendEmail = func(ctx context.Context, request ports.ProviderSendRequest) (*ports.ProviderSendResult, error) {
		return nil, errors.New("temporary provider error")
	}

	var retryEventSaved bool
	outbox := opts.OutboxWriter.(*mockOutboxWriter)
	outbox.saveFunc = func(ctx context.Context, event ports.OutboxEvent) error {
		if event.EventType == contracts.EventDeliveryMessageRetryScheduledV1 {
			retryEventSaved = true
		}
		return nil
	}

	svc := NewService(opts)
	_, err := svc.ProcessDueMessages(context.Background(), ProcessDueMessagesInput{
		WorkspaceID: "ws_1",
		MessageType: "marketing",
		Limit:       10,
		Now:         time.Now(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !retryCreated {
		t.Error("expected retry state to be created")
	}
	if !retryEventSaved {
		t.Error("expected retry scheduled outbox event")
	}
}

func TestProcessDueMessages_NoMessages(t *testing.T) {
	opts := newTestOpts()

	write := opts.MessagesWrite.(*mockMessageWriteRepo)
	write.claimDueMessages = func(ctx context.Context, query ports.DueMessageQuery, now time.Time) ([]domain.Message, error) {
		return nil, nil
	}

	svc := NewService(opts)
	result, err := svc.ProcessDueMessages(context.Background(), ProcessDueMessagesInput{
		WorkspaceID: "ws_1",
		MessageType: "marketing",
		Limit:       10,
		Now:         time.Now(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.SelectedCount != 0 {
		t.Errorf("expected 0 selected, got %d", result.SelectedCount)
	}
}

func TestProcessDueMessages_InvalidInput(t *testing.T) {
	svc := NewService(newTestOpts())

	_, err := svc.ProcessDueMessages(context.Background(), ProcessDueMessagesInput{
		WorkspaceID: "",
		MessageType: "marketing",
		Limit:       10,
		Now:         time.Now(),
	})
	if !errors.Is(err, domain.ErrPayloadInvalid) {
		t.Errorf("expected ErrPayloadInvalid, got %v", err)
	}

	_, err = svc.ProcessDueMessages(context.Background(), ProcessDueMessagesInput{
		WorkspaceID: "ws_1",
		MessageType: "unknown",
		Limit:       10,
		Now:         time.Now(),
	})
	if !errors.Is(err, domain.ErrPayloadInvalid) {
		t.Errorf("expected ErrPayloadInvalid for invalid message type, got %v", err)
	}
}
