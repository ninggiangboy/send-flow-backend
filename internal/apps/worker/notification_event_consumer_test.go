package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	identitycontracts "github.com/ninggiangboy/send-flow/backend/internal/modules/identity/contracts"
	notificationapp "github.com/ninggiangboy/send-flow/backend/internal/modules/notification/app"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/notification/domain"
	notificationports "github.com/ninggiangboy/send-flow/backend/internal/modules/notification/ports"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/events"
)

type mockNotifMsgWrite struct {
	create func(ctx context.Context, msg domain.NotificationMessage) error
	update func(ctx context.Context, msg domain.NotificationMessage) error
}

func (m *mockNotifMsgWrite) Create(ctx context.Context, msg domain.NotificationMessage) error {
	if m.create != nil {
		return m.create(ctx, msg)
	}
	return nil
}

func (m *mockNotifMsgWrite) Update(ctx context.Context, msg domain.NotificationMessage) error {
	if m.update != nil {
		return m.update(ctx, msg)
	}
	return nil
}

type mockNotifAttemptWrite struct {
	create func(ctx context.Context, attempt domain.NotificationAttempt) error
}

func (m *mockNotifAttemptWrite) Create(ctx context.Context, attempt domain.NotificationAttempt) error {
	if m.create != nil {
		return m.create(ctx, attempt)
	}
	return nil
}

type mockNotifEmailSender struct {
	send func(ctx context.Context, to []string, subject, textBody, htmlBody string) error
}

func (m *mockNotifEmailSender) SendNotificationEmail(ctx context.Context, to []string, subject, textBody, htmlBody string) error {
	if m.send != nil {
		return m.send(ctx, to, subject, textBody, htmlBody)
	}
	return nil
}

type mockNotifOutbox struct {
	save func(ctx context.Context, event notificationports.OutboxEvent) error
}

func (m *mockNotifOutbox) Save(ctx context.Context, event notificationports.OutboxEvent) error {
	if m.save != nil {
		return m.save(ctx, event)
	}
	return nil
}

type mockNotifTxManager struct {
	withinTx func(ctx context.Context, fn func(ctx context.Context) error) error
}

func (m *mockNotifTxManager) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if m.withinTx != nil {
		return m.withinTx(ctx, fn)
	}
	return fn(ctx)
}

func testNotificationConsumer(svc *notificationapp.Service) *NotificationEventConsumer {
	return NewNotificationEventConsumer(svc, testConsumerLogger(), nil, "", nil, "http://localhost:3000")
}

func validUserRegisteredEnvelope(t *testing.T) []byte {
	t.Helper()

	payload := identitycontracts.UserRegisteredPayload{
		UserID:     "user-1",
		Email:      "test@example.com",
		AuthMethod: "email",
		At:         time.Now().UTC().Format(time.RFC3339),
	}

	envelope, err := events.NewEnvelope(events.NewEnvelopeOptions{
		EventID:       "evt-1",
		EventType:     identitycontracts.EventUserRegisteredV1,
		EventVersion:  1,
		AggregateType: "user",
		AggregateID:   "user-1",
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

func validMemberInvitedEnvelope(t *testing.T) []byte {
	t.Helper()

	payload := identitycontracts.WorkspaceMemberInvitedPayload{
		WorkspaceID: "ws-1",
		Email:       "invited@example.com",
		Role:        "member",
		InvitedBy:   "user-1",
		At:          time.Now().UTC().Format(time.RFC3339),
	}

	envelope, err := events.NewEnvelope(events.NewEnvelopeOptions{
		EventID:       "evt-2",
		EventType:     identitycontracts.EventWorkspaceMemberInvitedV1,
		EventVersion:  1,
		AggregateType: "workspace_member",
		AggregateID:   "ws-1",
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

func minNotifServiceOpts() notificationapp.Options {
	idCounter := 0
	return notificationapp.Options{
		MessagesWrite: &mockNotifMsgWrite{},
		AttemptsWrite: &mockNotifAttemptWrite{},
		EmailSender:   &mockNotifEmailSender{},
		OutboxWriter:  &mockNotifOutbox{},
		TxManager:     &mockNotifTxManager{},
		IDGen: func() (string, error) {
			idCounter++
			return "id-gen", nil
		},
		Logger: testConsumerLogger(),
	}
}

func TestNotificationEventConsumer_ValidUserRegisteredEvent(t *testing.T) {
	svc := notificationapp.NewService(minNotifServiceOpts())
	consumer := testNotificationConsumer(svc)
	rawPayload := validUserRegisteredEnvelope(t)

	err := consumer.handleUserRegisteredEvent(context.Background(), "evt-1", rawPayload)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestNotificationEventConsumer_ValidMemberInvitedEvent(t *testing.T) {
	svc := notificationapp.NewService(minNotifServiceOpts())
	consumer := testNotificationConsumer(svc)
	rawPayload := validMemberInvitedEnvelope(t)

	err := consumer.handleMemberInvitedEvent(context.Background(), "evt-2", rawPayload)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestNotificationEventConsumer_WrongEventTypeIsHandled(t *testing.T) {
	svc := notificationapp.NewService(minNotifServiceOpts())
	consumer := testNotificationConsumer(svc)

	payload := identitycontracts.UserRegisteredPayload{
		UserID:     "user-1",
		Email:      "test@example.com",
		AuthMethod: "email",
		At:         time.Now().UTC().Format(time.RFC3339),
	}

	envelope, err := events.NewEnvelope(events.NewEnvelopeOptions{
		EventID:       "evt-1",
		EventType:     "some.other.event.v1",
		EventVersion:  1,
		AggregateType: "user",
		AggregateID:   "user-1",
		OccurredAt:    time.Now().UTC(),
	}, payload)
	if err != nil {
		t.Fatal(err)
	}
	rawPayload, err := events.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}

	err = consumer.handleUserRegisteredEvent(context.Background(), "evt-1", rawPayload)
	if err != nil {
		t.Fatalf("expected no error for wrong event type (handler does not check event type), got %v", err)
	}
}

func TestNotificationEventConsumer_MalformedPayload(t *testing.T) {
	svc := notificationapp.NewService(notificationapp.Options{
		Logger: testConsumerLogger(),
	})
	consumer := testNotificationConsumer(svc)

	err := consumer.handleUserRegisteredEvent(context.Background(), "evt-1", []byte("not json"))
	var nonRetryable *notificationapp.NonRetryableError
	if !errors.As(err, &nonRetryable) {
		t.Fatalf("expected NonRetryableError, got %v", err)
	}
}

func TestNotificationEventConsumer_RetryableError(t *testing.T) {
	wantErr := errors.New("database connection failed")
	svc := notificationapp.NewService(notificationapp.Options{
		MessagesWrite: &mockNotifMsgWrite{
			create: func(ctx context.Context, msg domain.NotificationMessage) error {
				return wantErr
			},
		},
		AttemptsWrite: &mockNotifAttemptWrite{},
		EmailSender:   &mockNotifEmailSender{},
		OutboxWriter:  &mockNotifOutbox{},
		TxManager:     &mockNotifTxManager{},
		IDGen: func() (string, error) {
			return "id-gen", nil
		},
		Logger: testConsumerLogger(),
	})
	consumer := testNotificationConsumer(svc)
	rawPayload := validUserRegisteredEnvelope(t)

	err := consumer.handleUserRegisteredEvent(context.Background(), "evt-1", rawPayload)
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected retryable error %v, got %v", wantErr, err)
	}
}
