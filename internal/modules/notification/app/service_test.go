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
	create func(ctx context.Context, msg domain.NotificationMessage) error
	update func(ctx context.Context, msg domain.NotificationMessage) error
}

func (m *mockMessageWriteRepo) Create(ctx context.Context, msg domain.NotificationMessage) error {
	return m.create(ctx, msg)
}

func (m *mockMessageWriteRepo) Update(ctx context.Context, msg domain.NotificationMessage) error {
	return m.update(ctx, msg)
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

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestSendWelcomeEmail_CreatesMessageAndSends(t *testing.T) {
	emailSent := false
	var createdMsg *domain.NotificationMessage

	svc := NewService(Options{
		MessagesRead: &mockMessageReadRepo{},
		MessagesWrite: &mockMessageWriteRepo{
			create: func(ctx context.Context, msg domain.NotificationMessage) error {
				createdMsg = &msg
				return nil
			},
			update: func(ctx context.Context, msg domain.NotificationMessage) error {
				return nil
			},
		},
		AttemptsWrite: &mockAttemptWriteRepo{
			create: func(ctx context.Context, attempt domain.NotificationAttempt) error {
				return nil
			},
		},
		OutboxWriter: &mockOutbox{
			save: func(ctx context.Context, event ports.OutboxEvent) error {
				return nil
			},
		},
		TxManager: &mockTxManager{},
		EmailSender: &mockEmailSender{
			sendNotificationEmail: func(ctx context.Context, to []string, subject, textBody, htmlBody string) error {
				emailSent = true
				return nil
			},
		},
		IDGen:  func() (string, error) { return "msg_1", nil },
		Logger: testLogger(),
	})

	msg, err := svc.SendWelcomeEmail(context.Background(), domain.SendWelcomeEmailInput{
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

func TestSendWelcomeEmail_EmailSendFails_Retries(t *testing.T) {
	var updatedMsg *domain.NotificationMessage
	emailSendCount := 0

	svc := NewService(Options{
		MessagesRead: &mockMessageReadRepo{},
		MessagesWrite: &mockMessageWriteRepo{
			create: func(ctx context.Context, msg domain.NotificationMessage) error {
				return nil
			},
			update: func(ctx context.Context, msg domain.NotificationMessage) error {
				updatedMsg = &msg
				return nil
			},
		},
		AttemptsWrite: &mockAttemptWriteRepo{
			create: func(ctx context.Context, attempt domain.NotificationAttempt) error {
				return nil
			},
		},
		OutboxWriter: &mockOutbox{
			save: func(ctx context.Context, event ports.OutboxEvent) error {
				return nil
			},
		},
		TxManager: &mockTxManager{},
		EmailSender: &mockEmailSender{
			sendNotificationEmail: func(ctx context.Context, to []string, subject, textBody, htmlBody string) error {
				emailSendCount++
				return errors.New("smtp timeout")
			},
		},
		IDGen:  func() (string, error) { return "msg_2", nil },
		Logger: testLogger(),
	})

	msg, err := svc.SendWelcomeEmail(context.Background(), domain.SendWelcomeEmailInput{
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

func TestSendInvitationEmail_CreatesMessageAndSends(t *testing.T) {
	emailSent := false

	svc := NewService(Options{
		MessagesRead: &mockMessageReadRepo{},
		MessagesWrite: &mockMessageWriteRepo{
			create: func(ctx context.Context, msg domain.NotificationMessage) error {
				return nil
			},
			update: func(ctx context.Context, msg domain.NotificationMessage) error {
				return nil
			},
		},
		AttemptsWrite: &mockAttemptWriteRepo{
			create: func(ctx context.Context, attempt domain.NotificationAttempt) error {
				return nil
			},
		},
		OutboxWriter: &mockOutbox{
			save: func(ctx context.Context, event ports.OutboxEvent) error {
				return nil
			},
		},
		TxManager: &mockTxManager{},
		EmailSender: &mockEmailSender{
			sendNotificationEmail: func(ctx context.Context, to []string, subject, textBody, htmlBody string) error {
				emailSent = true
				return nil
			},
		},
		IDGen:  func() (string, error) { return "msg_3", nil },
		Logger: testLogger(),
	})

	msg, err := svc.SendWorkspaceInvitationEmail(context.Background(), domain.SendInvitationEmailInput{
		WorkspaceID:     "ws_1",
		InvitedByEmail:  "admin@example.com",
		InvitedByUserID: "user_admin",
		InviteeEmail:    "invitee@example.com",
		Role:            "member",
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
}

func TestSendSystemAlert_CreatesMessageAndSends(t *testing.T) {
	emailSent := false

	svc := NewService(Options{
		MessagesRead: &mockMessageReadRepo{},
		MessagesWrite: &mockMessageWriteRepo{
			create: func(ctx context.Context, msg domain.NotificationMessage) error {
				return nil
			},
			update: func(ctx context.Context, msg domain.NotificationMessage) error {
				return nil
			},
		},
		AttemptsWrite: &mockAttemptWriteRepo{
			create: func(ctx context.Context, attempt domain.NotificationAttempt) error {
				return nil
			},
		},
		OutboxWriter: &mockOutbox{
			save: func(ctx context.Context, event ports.OutboxEvent) error {
				return nil
			},
		},
		TxManager: &mockTxManager{},
		EmailSender: &mockEmailSender{
			sendNotificationEmail: func(ctx context.Context, to []string, subject, textBody, htmlBody string) error {
				emailSent = true
				return nil
			},
		},
		IDGen:  func() (string, error) { return "msg_4", nil },
		Logger: testLogger(),
	})

	msg, err := svc.SendSystemAlert(context.Background(), domain.SendSystemAlertInput{
		RecipientEmail: "operator@example.com",
		Subject:        "System Alert",
		Body:           "Something went wrong",
		WorkspaceID:    "ws_1",
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
}

func TestGetNotificationStatus_RequiresPermission(t *testing.T) {
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

func TestListNotifications_FiltersByWorkspace(t *testing.T) {
	svc := NewService(Options{
		MessagesRead: &mockMessageReadRepo{
			list: func(ctx context.Context, filter domain.NotificationFilter) ([]domain.NotificationMessage, string, error) {
				if filter.WorkspaceID == nil || *filter.WorkspaceID != "ws_1" {
					t.Errorf("expected workspace filter ws_1")
				}
				return []domain.NotificationMessage{}, "", nil
			},
		},
		Logger: testLogger(),
	})

	ws := "ws_1"
	result, err := svc.ListNotifications(context.Background(), domain.NotificationFilter{
		WorkspaceID: &ws,
		Limit:       50,
	}, "user_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
}

func TestInvalidRecipientEmail_ReturnsError(t *testing.T) {
	svc := NewService(Options{
		Logger: testLogger(),
	})

	_, err := svc.SendWelcomeEmail(context.Background(), domain.SendWelcomeEmailInput{
		UserID:          "user_1",
		Email:           "invalid",
		FrontendBaseURL: "http://localhost:3000",
	})
	if !errors.Is(err, domain.ErrRecipientEmailInvalid) {
		t.Fatalf("expected ErrRecipientEmailInvalid, got %v", err)
	}
}
