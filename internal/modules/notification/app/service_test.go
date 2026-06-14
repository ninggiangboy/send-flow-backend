package app

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/notification/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/notification/ports"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type mockMessageReadRepo struct {
	ports.MessageReadRepository
	findByID            func(ctx context.Context, id string) (*domain.NotificationMessage, error)
	list                func(ctx context.Context, filter domain.NotificationFilter) ([]domain.NotificationMessage, string, error)
	findPendingForRetry func(ctx context.Context, limit int) ([]domain.NotificationMessage, error)
}

func (m *mockMessageReadRepo) FindByID(ctx context.Context, id string) (*domain.NotificationMessage, error) {
	return m.findByID(ctx, id)
}
func (m *mockMessageReadRepo) List(ctx context.Context, filter domain.NotificationFilter) ([]domain.NotificationMessage, string, error) {
	return m.list(ctx, filter)
}
func (m *mockMessageReadRepo) FindPendingForRetry(ctx context.Context, limit int) ([]domain.NotificationMessage, error) {
	return m.findPendingForRetry(ctx, limit)
}

type mockMessageWriteRepo struct {
	ports.MessageWriteRepository
	create                func(ctx context.Context, msg domain.NotificationMessage) error
	update                func(ctx context.Context, msg domain.NotificationMessage) error
	claimRetryingMessages func(ctx context.Context, limit int) ([]domain.NotificationMessage, error)
}

func (m *mockMessageWriteRepo) Create(ctx context.Context, msg domain.NotificationMessage) error {
	return m.create(ctx, msg)
}
func (m *mockMessageWriteRepo) Update(ctx context.Context, msg domain.NotificationMessage) error {
	return m.update(ctx, msg)
}
func (m *mockMessageWriteRepo) ClaimRetryingMessages(ctx context.Context, limit int) ([]domain.NotificationMessage, error) {
	return m.claimRetryingMessages(ctx, limit)
}

type mockAttemptReadRepo struct {
	ports.AttemptReadRepository
	findByMessageID func(ctx context.Context, notificationMessageID string) ([]domain.NotificationAttempt, error)
}

func (m *mockAttemptReadRepo) FindByMessageID(ctx context.Context, notificationMessageID string) ([]domain.NotificationAttempt, error) {
	return m.findByMessageID(ctx, notificationMessageID)
}

type mockAttemptWriteRepo struct {
	ports.AttemptWriteRepository
	create func(ctx context.Context, attempt domain.NotificationAttempt) error
}

func (m *mockAttemptWriteRepo) Create(ctx context.Context, attempt domain.NotificationAttempt) error {
	return m.create(ctx, attempt)
}

type mockOutbox struct {
	ports.OutboxWriter
	save func(ctx context.Context, event ports.OutboxEvent) error
}

func (m *mockOutbox) Save(ctx context.Context, event ports.OutboxEvent) error {
	return m.save(ctx, event)
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

type mockAccessChecker struct {
	ports.WorkspaceAccessChecker
	requirePermission func(ctx context.Context, workspaceID, userID, permission string) error
}

func (m *mockAccessChecker) RequirePermission(ctx context.Context, workspaceID, userID, permission string) error {
	return m.requirePermission(ctx, workspaceID, userID, permission)
}

type mockEmailSender struct {
	ports.EmailSender
	sendNotificationEmail func(ctx context.Context, to []string, subject, textBody, htmlBody string) error
}

func (m *mockEmailSender) SendNotificationEmail(ctx context.Context, to []string, subject, textBody, htmlBody string) error {
	return m.sendNotificationEmail(ctx, to, subject, textBody, htmlBody)
}

func TestServiceSendWelcomeEmail_DelegatesToHandler(t *testing.T) {
	called := false
	svc := NewService(Options{
		MessagesRead: &mockMessageReadRepo{},
		MessagesWrite: &mockMessageWriteRepo{
			create: func(ctx context.Context, msg domain.NotificationMessage) error { called = true; return nil },
			update: func(ctx context.Context, msg domain.NotificationMessage) error { return nil },
		},
		AttemptsWrite: &mockAttemptWriteRepo{
			create: func(ctx context.Context, attempt domain.NotificationAttempt) error { return nil },
		},
		OutboxWriter: &mockOutbox{
			save: func(ctx context.Context, event ports.OutboxEvent) error { return nil },
		},
		TxManager: &mockTxManager{},
		EmailSender: &mockEmailSender{
			sendNotificationEmail: func(ctx context.Context, to []string, subject, textBody, htmlBody string) error {
				return nil
			},
		},
		IDGen:  func() (string, error) { return "msg_1", nil },
		Logger: testLogger(),
	})

	_, err := svc.SendWelcomeEmail(context.Background(), domain.SendWelcomeEmailInput{
		UserID:          "user_1",
		Email:           "test@example.com",
		FrontendBaseURL: "http://localhost:3000",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("expected handler to be called via facade")
	}
}

func TestServiceGetNotificationStatus_ReadsDenied(t *testing.T) {
	svc := NewService(Options{
		AccessChecker: &mockAccessChecker{
			requirePermission: func(ctx context.Context, workspaceID, userID, permission string) error {
				return domain.ErrNotificationReadDenied
			},
		},
		MessagesRead: &mockMessageReadRepo{},
		AttemptsRead: &mockAttemptReadRepo{},
		Logger:       testLogger(),
	})

	_, err := svc.GetNotificationStatus(context.Background(), "ws_1", "msg_1", "user_1")
	if !errors.Is(err, domain.ErrNotificationReadDenied) {
		t.Fatalf("expected ErrNotificationReadDenied, got %v", err)
	}
}

func TestServiceSendSystemAlert_DelegatesToHandler(t *testing.T) {
	called := false
	svc := NewService(Options{
		MessagesRead: &mockMessageReadRepo{},
		MessagesWrite: &mockMessageWriteRepo{
			create: func(ctx context.Context, msg domain.NotificationMessage) error { called = true; return nil },
			update: func(ctx context.Context, msg domain.NotificationMessage) error { return nil },
		},
		AttemptsWrite: &mockAttemptWriteRepo{
			create: func(ctx context.Context, attempt domain.NotificationAttempt) error { return nil },
		},
		OutboxWriter: &mockOutbox{
			save: func(ctx context.Context, event ports.OutboxEvent) error { return nil },
		},
		TxManager: &mockTxManager{},
		EmailSender: &mockEmailSender{
			sendNotificationEmail: func(ctx context.Context, to []string, subject, textBody, htmlBody string) error {
				return nil
			},
		},
		IDGen:  func() (string, error) { return "msg_3", nil },
		Logger: testLogger(),
	})

	_, err := svc.SendSystemAlert(context.Background(), domain.SendSystemAlertInput{
		RecipientEmail: "alert@example.com",
		Subject:        "Alert",
		Body:           "Something",
		WorkspaceID:    "ws_1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("expected handler to be called via facade")
	}
}

func TestServiceProcessRetryBatch_DelegatesToHandler(t *testing.T) {
	svc := NewService(Options{
		MessagesRead: &mockMessageReadRepo{},
		MessagesWrite: &mockMessageWriteRepo{
			create: func(ctx context.Context, msg domain.NotificationMessage) error { return nil },
			update: func(ctx context.Context, msg domain.NotificationMessage) error { return nil },
			claimRetryingMessages: func(ctx context.Context, limit int) ([]domain.NotificationMessage, error) {
				return nil, nil
			},
		},
		AttemptsWrite: &mockAttemptWriteRepo{
			create: func(ctx context.Context, attempt domain.NotificationAttempt) error { return nil },
		},
		OutboxWriter: &mockOutbox{
			save: func(ctx context.Context, event ports.OutboxEvent) error { return nil },
		},
		TxManager: &mockTxManager{},
		EmailSender: &mockEmailSender{
			sendNotificationEmail: func(ctx context.Context, to []string, subject, textBody, htmlBody string) error {
				return nil
			},
		},
		IDGen:  func() (string, error) { return "msg_4", nil },
		Logger: testLogger(),
	})

	count, err := svc.ProcessRetryBatch(context.Background(), 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 processed, got %d", count)
	}
}
