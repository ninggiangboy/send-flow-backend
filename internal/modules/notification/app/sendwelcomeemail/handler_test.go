package sendwelcomeemail

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/notification/app/usecase"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/notification/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/notification/ports"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type mockMessagesWrite struct {
	ports.MessageWriteRepository
	create                func(ctx context.Context, msg domain.NotificationMessage) error
	update                func(ctx context.Context, msg domain.NotificationMessage) error
	claimRetryingMessages func(ctx context.Context, limit int) ([]domain.NotificationMessage, error)
}

func (m *mockMessagesWrite) Create(ctx context.Context, msg domain.NotificationMessage) error {
	return m.create(ctx, msg)
}
func (m *mockMessagesWrite) Update(ctx context.Context, msg domain.NotificationMessage) error {
	return m.update(ctx, msg)
}

type mockTxManager struct {
	ports.TransactionManager
	withinTx func(ctx context.Context, fn func(ctx context.Context) error) error
}

func (m *mockTxManager) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if m.withinTx != nil {
		return m.withinTx(ctx, fn)
	}
	return fn(ctx)
}

type mockOutboxWriter struct {
	ports.OutboxWriter
	save func(ctx context.Context, event ports.OutboxEvent) error
}

func (m *mockOutboxWriter) Save(ctx context.Context, event ports.OutboxEvent) error {
	return m.save(ctx, event)
}

type mockAttemptsWrite struct {
	ports.AttemptWriteRepository
	create func(ctx context.Context, attempt domain.NotificationAttempt) error
}

func (m *mockAttemptsWrite) Create(ctx context.Context, attempt domain.NotificationAttempt) error {
	return m.create(ctx, attempt)
}

type mockEmailSenderPort struct {
	ports.EmailSender
	sendNotificationEmail func(ctx context.Context, to []string, subject, textBody, htmlBody string) error
}

func (m *mockEmailSenderPort) SendNotificationEmail(ctx context.Context, to []string, subject, textBody, htmlBody string) error {
	return m.sendNotificationEmail(ctx, to, subject, textBody, htmlBody)
}

func TestExecute_CreatesMessageAndSends(t *testing.T) {
	emailSent := false
	var createdMsg *domain.NotificationMessage

	handler := newTestHandler(&testHandlerOpts{
		create: func(ctx context.Context, msg domain.NotificationMessage) error {
			createdMsg = &msg
			return nil
		},
		update: func(ctx context.Context, msg domain.NotificationMessage) error { return nil },
		sendEmail: func(ctx context.Context, to []string, subject, textBody, htmlBody string) error {
			emailSent = true
			return nil
		},
	})

	msg, err := handler.Execute(context.Background(), domain.SendWelcomeEmailInput{
		UserID:          "user_1",
		Email:           "test@example.com",
		FrontendBaseURL: "http://localhost:3000",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg == nil {
		t.Fatal("expected message, got nil")
	}
	if !emailSent {
		t.Fatal("expected email to be sent")
	}
	if createdMsg == nil {
		t.Fatal("expected message to be created")
	}
	if createdMsg.Type != domain.NotificationTypeWelcomeEmail {
		t.Errorf("expected type welcome_email, got %s", createdMsg.Type)
	}
}

func TestExecute_EmailSendFails_Retries(t *testing.T) {
	var updatedMsg *domain.NotificationMessage
	emailSendCount := 0

	handler := newTestHandler(&testHandlerOpts{
		create: func(ctx context.Context, msg domain.NotificationMessage) error { return nil },
		update: func(ctx context.Context, msg domain.NotificationMessage) error {
			updatedMsg = &msg
			return nil
		},
		sendEmail: func(ctx context.Context, to []string, subject, textBody, htmlBody string) error {
			emailSendCount++
			return errors.New("smtp timeout")
		},
	})

	msg, err := handler.Execute(context.Background(), domain.SendWelcomeEmailInput{
		UserID:          "user_2",
		Email:           "test@example.com",
		FrontendBaseURL: "http://localhost:3000",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg == nil {
		t.Fatal("expected message, got nil")
	}
	if emailSendCount != 1 {
		t.Fatalf("expected 1 email send attempt, got %d", emailSendCount)
	}
	if updatedMsg == nil {
		t.Fatal("expected update to be called")
	}
	if updatedMsg.Status != domain.NotificationStatusRetrying {
		t.Errorf("expected status retrying, got %s", updatedMsg.Status)
	}
}

func TestExecute_InvalidRecipientEmail_ReturnsError(t *testing.T) {
	handler := newTestHandler(&testHandlerOpts{})

	_, err := handler.Execute(context.Background(), domain.SendWelcomeEmailInput{
		UserID:          "user_1",
		Email:           "invalid",
		FrontendBaseURL: "http://localhost:3000",
	})
	if !errors.Is(err, domain.ErrRecipientEmailInvalid) {
		t.Fatalf("expected ErrRecipientEmailInvalid, got %v", err)
	}
}

type testHandlerOpts struct {
	create    func(ctx context.Context, msg domain.NotificationMessage) error
	update    func(ctx context.Context, msg domain.NotificationMessage) error
	sendEmail func(ctx context.Context, to []string, subject, textBody, htmlBody string) error
}

func newTestHandler(opts *testHandlerOpts) *Handler {
	mw := &mockMessagesWrite{
		create: opts.create,
		update: opts.update,
	}
	if mw.create == nil {
		mw.create = func(ctx context.Context, msg domain.NotificationMessage) error { return nil }
	}
	if mw.update == nil {
		mw.update = func(ctx context.Context, msg domain.NotificationMessage) error { return nil }
	}

	mep := &mockEmailSenderPort{sendNotificationEmail: opts.sendEmail}
	if mep.sendNotificationEmail == nil {
		mep.sendNotificationEmail = func(ctx context.Context, to []string, subject, textBody, htmlBody string) error { return nil }
	}

	mow := &mockOutboxWriter{
		save: func(ctx context.Context, event ports.OutboxEvent) error { return nil },
	}
	maw := &mockAttemptsWrite{
		create: func(ctx context.Context, attempt domain.NotificationAttempt) error { return nil },
	}

	idGen := func() (string, error) { return "msg_1", nil }
	eventPub := usecase.NewEventPublisher(mow, idGen)
	emailSender := usecase.NewEmailSender(mw, maw, mep, eventPub, idGen, testLogger())

	return New(Options{
		MessagesWrite: mw,
		TxManager:     &mockTxManager{},
		EmailSender:   emailSender,
		EventPub:      eventPub,
		IDGen:         idGen,
		Logger:        testLogger(),
	})
}
