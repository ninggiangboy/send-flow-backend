package send

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/notification/app/shared"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/notification/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/notification/ports"
)

func notificationTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type notificationMessagesWriteStub struct {
	create func(context.Context, domain.NotificationMessage) error
	update func(context.Context, domain.NotificationMessage) error
}

func (m *notificationMessagesWriteStub) Create(ctx context.Context, msg domain.NotificationMessage) error {
	if m.create != nil {
		return m.create(ctx, msg)
	}
	return nil
}
func (m *notificationMessagesWriteStub) Update(ctx context.Context, msg domain.NotificationMessage) error {
	if m.update != nil {
		return m.update(ctx, msg)
	}
	return nil
}
func (m *notificationMessagesWriteStub) FindByID(context.Context, string) (*domain.NotificationMessage, error) {
	return nil, nil
}
func (m *notificationMessagesWriteStub) List(context.Context, domain.NotificationFilter) ([]domain.NotificationMessage, string, error) {
	return nil, "", nil
}
func (m *notificationMessagesWriteStub) FindPendingForRetry(context.Context, int) ([]domain.NotificationMessage, error) {
	return nil, nil
}
func (m *notificationMessagesWriteStub) ClaimRetryingMessages(context.Context, int) ([]domain.NotificationMessage, error) {
	return nil, nil
}

type notificationTxManagerStub struct{}

func (notificationTxManagerStub) WithinTx(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

type notificationOutboxStub struct{}

func (notificationOutboxStub) Save(context.Context, ports.OutboxEvent) error { return nil }

type notificationAttemptsWriteStub struct{}

func (notificationAttemptsWriteStub) Create(context.Context, domain.NotificationAttempt) error {
	return nil
}
func (notificationAttemptsWriteStub) FindByMessageID(context.Context, string) ([]domain.NotificationAttempt, error) {
	return nil, nil
}

type notificationEmailSenderStub struct {
	send func(context.Context, []string, string, string, string) error
}

func (m *notificationEmailSenderStub) SendNotificationEmail(ctx context.Context, to []string, subject, textBody, htmlBody string) error {
	return m.send(ctx, to, subject, textBody, htmlBody)
}

func TestWelcomeHandlerExecuteCreatesAndSends(t *testing.T) {
	var created *domain.NotificationMessage
	emailSent := false
	messagesWrite := &notificationMessagesWriteStub{
		create: func(_ context.Context, msg domain.NotificationMessage) error { created = &msg; return nil },
	}
	emailSender := shared.NewEmailSender(
		messagesWrite,
		notificationAttemptsWriteStub{},
		&notificationEmailSenderStub{send: func(context.Context, []string, string, string, string) error { emailSent = true; return nil }},
		shared.NewEventPublisher(notificationOutboxStub{}, func() (string, error) { return "msg_1", nil }),
		func() (string, error) { return "msg_1", nil },
		notificationTestLogger(),
	)

	h := NewWelcome(WelcomeOptions{
		MessagesWrite: messagesWrite,
		TxManager:     notificationTxManagerStub{},
		EmailSender:   emailSender,
		EventPub:      shared.NewEventPublisher(notificationOutboxStub{}, func() (string, error) { return "msg_1", nil }),
		IDGen:         func() (string, error) { return "msg_1", nil },
		Logger:        notificationTestLogger(),
	})

	msg, err := h.Execute(context.Background(), domain.SendWelcomeEmailInput{UserID: "user_1", Email: "test@example.com", FrontendBaseURL: "http://localhost:3000"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg == nil || created == nil || !emailSent {
		t.Fatalf("expected message create/send, got msg=%v created=%v emailSent=%v", msg != nil, created != nil, emailSent)
	}
}

func TestWelcomeHandlerExecuteInvalidRecipientEmail(t *testing.T) {
	h := NewWelcome(WelcomeOptions{
		MessagesWrite: &notificationMessagesWriteStub{},
		TxManager:     notificationTxManagerStub{},
		EmailSender: shared.NewEmailSender(
			&notificationMessagesWriteStub{},
			notificationAttemptsWriteStub{},
			&notificationEmailSenderStub{send: func(context.Context, []string, string, string, string) error { return nil }},
			shared.NewEventPublisher(notificationOutboxStub{}, func() (string, error) { return "msg_1", nil }),
			func() (string, error) { return "msg_1", nil },
			notificationTestLogger(),
		),
		EventPub: shared.NewEventPublisher(notificationOutboxStub{}, func() (string, error) { return "msg_1", nil }),
		IDGen:    func() (string, error) { return "msg_1", nil },
		Logger:   notificationTestLogger(),
	})

	_, err := h.Execute(context.Background(), domain.SendWelcomeEmailInput{UserID: "user_1", Email: "invalid", FrontendBaseURL: "http://localhost:3000"})
	if !errors.Is(err, domain.ErrRecipientEmailInvalid) {
		t.Fatalf("expected ErrRecipientEmailInvalid, got %v", err)
	}
}
