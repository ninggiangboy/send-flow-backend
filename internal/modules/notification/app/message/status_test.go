package message

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/notification/domain"
)

func messageTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type messageReadStub struct {
	findByID func(context.Context, string) (*domain.NotificationMessage, error)
}

func (s *messageReadStub) FindByID(ctx context.Context, id string) (*domain.NotificationMessage, error) {
	return s.findByID(ctx, id)
}
func (s *messageReadStub) List(context.Context, domain.NotificationFilter) ([]domain.NotificationMessage, string, error) {
	return nil, "", nil
}
func (s *messageReadStub) FindPendingForRetry(context.Context, int) ([]domain.NotificationMessage, error) {
	return nil, nil
}

type attemptReadStub struct {
	findByMessageID func(context.Context, string) ([]domain.NotificationAttempt, error)
}

func (s *attemptReadStub) FindByMessageID(ctx context.Context, id string) ([]domain.NotificationAttempt, error) {
	return s.findByMessageID(ctx, id)
}

type notificationAccessCheckerStub struct {
	requirePermission func(context.Context, string, string, string) error
}

func (s *notificationAccessCheckerStub) RequirePermission(ctx context.Context, workspaceID, userID, permission string) error {
	return s.requirePermission(ctx, workspaceID, userID, permission)
}

func TestStatusHandlerExecuteReadsMessageAndAttempts(t *testing.T) {
	h := NewStatus(StatusOptions{
		MessagesRead: &messageReadStub{findByID: func(context.Context, string) (*domain.NotificationMessage, error) {
			return &domain.NotificationMessage{ID: "msg_1"}, nil
		}},
		AttemptsRead: &attemptReadStub{findByMessageID: func(context.Context, string) ([]domain.NotificationAttempt, error) {
			return []domain.NotificationAttempt{{ID: "attempt_1"}}, nil
		}},
		AccessChecker: &notificationAccessCheckerStub{requirePermission: func(context.Context, string, string, string) error { return nil }},
		Logger:        messageTestLogger(),
	})

	result, err := h.Execute(context.Background(), StatusQuery{WorkspaceID: "ws_1", MessageID: "msg_1", UserID: "user_1"})
	if err != nil || result.Message.ID != "msg_1" || len(result.Attempts) != 1 {
		t.Fatalf("unexpected result: %+v err=%v", result, err)
	}
}

func TestStatusHandlerExecuteReadDenied(t *testing.T) {
	h := NewStatus(StatusOptions{
		MessagesRead:  &messageReadStub{findByID: func(context.Context, string) (*domain.NotificationMessage, error) { return nil, nil }},
		AttemptsRead:  &attemptReadStub{findByMessageID: func(context.Context, string) ([]domain.NotificationAttempt, error) { return nil, nil }},
		AccessChecker: &notificationAccessCheckerStub{requirePermission: func(context.Context, string, string, string) error { return domain.ErrNotificationReadDenied }},
		Logger:        messageTestLogger(),
	})

	_, err := h.Execute(context.Background(), StatusQuery{WorkspaceID: "ws_1", MessageID: "msg_1", UserID: "user_1"})
	if !errors.Is(err, domain.ErrNotificationReadDenied) {
		t.Fatalf("expected ErrNotificationReadDenied, got %v", err)
	}
}
