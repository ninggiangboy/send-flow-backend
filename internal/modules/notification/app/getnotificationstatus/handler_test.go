package getnotificationstatus

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

type mockMessagesRead struct {
	ports.MessageReadRepository
	findByID func(ctx context.Context, id string) (*domain.NotificationMessage, error)
}

func (m *mockMessagesRead) FindByID(ctx context.Context, id string) (*domain.NotificationMessage, error) {
	return m.findByID(ctx, id)
}

type mockAttemptsRead struct {
	ports.AttemptReadRepository
	findByMessageID func(ctx context.Context, notificationMessageID string) ([]domain.NotificationAttempt, error)
}

func (m *mockAttemptsRead) FindByMessageID(ctx context.Context, notificationMessageID string) ([]domain.NotificationAttempt, error) {
	return m.findByMessageID(ctx, notificationMessageID)
}

type mockAccessChecker struct {
	ports.WorkspaceAccessChecker
	requirePermission func(ctx context.Context, workspaceID, userID, permission string) error
}

func (m *mockAccessChecker) RequirePermission(ctx context.Context, workspaceID, userID, permission string) error {
	return m.requirePermission(ctx, workspaceID, userID, permission)
}

func TestExecute_RequiresPermission(t *testing.T) {
	h := New(Options{
		AccessChecker: &mockAccessChecker{
			requirePermission: func(ctx context.Context, workspaceID, userID, permission string) error {
				return domain.ErrNotificationReadDenied
			},
		},
		MessagesRead: &mockMessagesRead{},
		AttemptsRead: &mockAttemptsRead{},
		Logger:       testLogger(),
	})

	_, err := h.Execute(context.Background(), Query{
		WorkspaceID: "ws_1",
		MessageID:   "msg_1",
		UserID:      "user_1",
	})
	if !errors.Is(err, domain.ErrNotificationReadDenied) {
		t.Fatalf("expected ErrNotificationReadDenied, got %v", err)
	}
}

func TestExecute_ReturnsMessageAndAttempts(t *testing.T) {
	h := New(Options{
		MessagesRead: &mockMessagesRead{
			findByID: func(ctx context.Context, id string) (*domain.NotificationMessage, error) {
				return &domain.NotificationMessage{ID: id, Status: domain.NotificationStatusSent}, nil
			},
		},
		AttemptsRead: &mockAttemptsRead{
			findByMessageID: func(ctx context.Context, id string) ([]domain.NotificationAttempt, error) {
				return []domain.NotificationAttempt{
					{ID: "attempt_1", NotificationMessageID: id, Status: domain.AttemptStatusSent},
				}, nil
			},
		},
		Logger: testLogger(),
	})

	result, err := h.Execute(context.Background(), Query{
		WorkspaceID: "ws_1",
		MessageID:   "msg_1",
		UserID:      "user_1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Message.ID != "msg_1" {
		t.Errorf("expected message id msg_1, got %s", result.Message.ID)
	}
	if len(result.Attempts) != 1 {
		t.Fatalf("expected 1 attempt, got %d", len(result.Attempts))
	}
	if result.Attempts[0].ID != "attempt_1" {
		t.Errorf("expected attempt id attempt_1, got %s", result.Attempts[0].ID)
	}
}
