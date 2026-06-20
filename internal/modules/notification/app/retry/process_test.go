package retry

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/notification/app/shared"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/notification/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/notification/ports"
)

func retryTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type retryMessagesWriteStub struct {
	claimed []domain.NotificationMessage
}

func (s *retryMessagesWriteStub) FindByID(context.Context, string) (*domain.NotificationMessage, error) {
	return nil, nil
}
func (s *retryMessagesWriteStub) List(context.Context, domain.NotificationFilter) ([]domain.NotificationMessage, string, error) {
	return nil, "", nil
}
func (s *retryMessagesWriteStub) FindPendingForRetry(context.Context, int) ([]domain.NotificationMessage, error) {
	return nil, nil
}
func (s *retryMessagesWriteStub) Create(context.Context, domain.NotificationMessage) error {
	return nil
}
func (s *retryMessagesWriteStub) Update(context.Context, domain.NotificationMessage) error {
	return nil
}
func (s *retryMessagesWriteStub) ClaimRetryingMessages(context.Context, int) ([]domain.NotificationMessage, error) {
	return s.claimed, nil
}

type retryAttemptsWriteStub struct{}

func (retryAttemptsWriteStub) FindByMessageID(context.Context, string) ([]domain.NotificationAttempt, error) {
	return nil, nil
}
func (retryAttemptsWriteStub) Create(context.Context, domain.NotificationAttempt) error { return nil }

type retryEmailSenderPortStub struct{}

func (retryEmailSenderPortStub) SendNotificationEmail(context.Context, []string, string, string, string) error {
	return nil
}

type retryOutboxStub struct{}

func (retryOutboxStub) Save(context.Context, ports.OutboxEvent) error { return nil }

func TestProcessHandlerExecuteProcessesClaimedMessages(t *testing.T) {
	msgRepo := &retryMessagesWriteStub{claimed: []domain.NotificationMessage{{ID: "msg_1", RecipientEmail: "test@example.com", Subject: "Hello", BodyText: "Hi", Status: domain.NotificationStatusRetrying, AttemptCount: 0, MaxAttempts: 3}}}
	emailSender := shared.NewEmailSender(
		msgRepo,
		retryAttemptsWriteStub{},
		retryEmailSenderPortStub{},
		shared.NewEventPublisher(retryOutboxStub{}, func() (string, error) { return "msg_1", nil }),
		func() (string, error) { return "msg_1", nil },
		retryTestLogger(),
	)

	handler := NewProcess(ProcessOptions{
		MessagesWrite: msgRepo,
		EmailSender:   emailSender,
		IDGen:         func() (string, error) { return "msg_1", nil },
		Logger:        retryTestLogger(),
	})

	processed, err := handler.Execute(context.Background(), 0)
	if err != nil || processed != 1 {
		t.Fatalf("unexpected result: processed=%d err=%v", processed, err)
	}
}
