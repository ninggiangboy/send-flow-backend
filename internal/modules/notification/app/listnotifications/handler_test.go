package listnotifications

import (
	"context"
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
	list func(ctx context.Context, filter domain.NotificationFilter) ([]domain.NotificationMessage, string, error)
}

func (m *mockMessagesRead) List(ctx context.Context, filter domain.NotificationFilter) ([]domain.NotificationMessage, string, error) {
	return m.list(ctx, filter)
}

func TestExecute_FiltersByWorkspace(t *testing.T) {
	h := New(Options{
		MessagesRead: &mockMessagesRead{
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
	result, err := h.Execute(context.Background(), Query{
		Filter: domain.NotificationFilter{
			WorkspaceID: &ws,
			Limit:       50,
		},
		UserID: "user_1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
}

func TestExecute_DefaultLimit(t *testing.T) {
	h := New(Options{
		MessagesRead: &mockMessagesRead{
			list: func(ctx context.Context, filter domain.NotificationFilter) ([]domain.NotificationMessage, string, error) {
				if filter.Limit != 50 {
					t.Errorf("expected limit 50, got %d", filter.Limit)
				}
				return []domain.NotificationMessage{}, "", nil
			},
		},
		Logger: testLogger(),
	})

	result, err := h.Execute(context.Background(), Query{
		Filter: domain.NotificationFilter{Limit: 0},
		UserID: "user_1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
}
