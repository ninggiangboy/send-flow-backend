package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"context"

	deliveryapp "github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/app"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/domain"
)

func TestMailLogs_List_APIKeyAuth(t *testing.T) {
	now := nowUTC()
	svc := &mockDeliverySvc{
		listMessagesFn: func(ctx context.Context, input deliveryapp.ListMessagesInput) (*deliveryapp.ListMessagesResult, error) {
			return &deliveryapp.ListMessagesResult{
				Messages: []domain.Message{
					{ID: "msg_1", WorkspaceID: "ws_1", Status: domain.MessageStatusDelivered, Subject: "Test", MessageType: domain.MessageTypeTransactional, CreatedAt: now},
				},
			}, nil
		},
	}
	h := &mailLogsHTTP{svc: svc}
	req := apiKeyContext(httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/ws_1/mail-logs", nil), "ws_1", "ak_1", []string{"mail_logs.read"})
	rec := httptest.NewRecorder()
	h.listMailLogs(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	data := resp["data"].(map[string]any)
	msgs := data["messages"].([]any)
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
}

func TestMailLogs_List_Filters(t *testing.T) {
	svc := &mockDeliverySvc{
		listMessagesFn: func(ctx context.Context, input deliveryapp.ListMessagesInput) (*deliveryapp.ListMessagesResult, error) {
			if input.Status != "delivered" {
				t.Fatalf("expected status filter delivered, got %s", input.Status)
			}
			if input.MessageType != "transactional" {
				t.Fatalf("expected message_type filter transactional, got %s", input.MessageType)
			}
			return &deliveryapp.ListMessagesResult{Messages: []domain.Message{}}, nil
		},
	}
	h := &mailLogsHTTP{svc: svc}
	req := apiKeyContext(httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/ws_1/mail-logs?status=delivered&message_type=transactional", nil), "ws_1", "ak_1", []string{"mail_logs.read"})
	rec := httptest.NewRecorder()
	h.listMailLogs(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestMailLogs_GetDetail_Success(t *testing.T) {
	now := nowUTC()
	svc := &mockDeliverySvc{
		getMessageFn: func(ctx context.Context, input deliveryapp.GetMessageInput) (*domain.Message, error) {
			return &domain.Message{
				ID:                       "msg_1",
				WorkspaceID:              "ws_1",
				RecipientEmailNormalized: "a@example.com",
				Status:                   domain.MessageStatusDelivered,
				Subject:                  "Test",
				SenderName:               "Alice",
				MessageType:              domain.MessageTypeTransactional,
				CreatedAt:                now,
				DeliveredAt:              &now,
			}, nil
		},
	}
	h := &mailLogsHTTP{svc: svc}
	req := apiKeyContext(httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/ws_1/mail-logs/msg_1", nil), "ws_1", "ak_1", []string{"mail_logs.read"})
	req = req.WithContext(chiCtx(req.Context(), "message_id", "msg_1"))
	rec := httptest.NewRecorder()
	h.getMailLog(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestMailLogs_GetDetail_NotFound(t *testing.T) {
	svc := &mockDeliverySvc{
		getMessageFn: func(ctx context.Context, input deliveryapp.GetMessageInput) (*domain.Message, error) {
			return nil, domain.ErrMessageNotFound
		},
	}
	h := &mailLogsHTTP{svc: svc}
	req := apiKeyContext(httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/ws_1/mail-logs/msg_notfound", nil), "ws_1", "ak_1", []string{"mail_logs.read"})
	req = req.WithContext(chiCtx(req.Context(), "message_id", "msg_notfound"))
	rec := httptest.NewRecorder()
	h.getMailLog(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestMailLogs_GetAttempts(t *testing.T) {
	now := nowUTC()
	svc := &mockDeliverySvc{
		listAttemptsFn: func(ctx context.Context, input deliveryapp.ListAttemptsInput) ([]domain.DeliveryAttempt, error) {
			return []domain.DeliveryAttempt{
				{ID: "att_1", AttemptNo: 1, Provider: "ses", Status: domain.MessageStatusDelivered, StartedAt: now},
			}, nil
		},
	}
	h := &mailLogsHTTP{svc: svc}
	req := apiKeyContext(httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/ws_1/mail-logs/msg_1/attempts", nil), "ws_1", "ak_1", []string{"mail_logs.read"})
	req = req.WithContext(chiCtx(req.Context(), "message_id", "msg_1"))
	rec := httptest.NewRecorder()
	h.listMailLogAttempts(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	data := resp["data"].(map[string]any)
	attempts := data["attempts"].([]any)
	if len(attempts) != 1 {
		t.Fatalf("expected 1 attempt, got %d", len(attempts))
	}
}

func TestMailLogs_GetEvents(t *testing.T) {
	now := nowUTC()
	svc := &mockDeliverySvc{
		listMessageEventsFn: func(ctx context.Context, input deliveryapp.ListMessageEventsInput) (*deliveryapp.ListMessageEventsResult, error) {
			return &deliveryapp.ListMessageEventsResult{
				Events: []domain.MessageEvent{
					{ID: "evt_1", EventType: domain.MessageEventQueued, Status: domain.MessageStatusQueued, OccurredAt: now},
					{ID: "evt_2", EventType: domain.MessageEventDelivered, Status: domain.MessageStatusDelivered, OccurredAt: now},
				},
			}, nil
		},
	}
	h := &mailLogsHTTP{svc: svc}
	req := apiKeyContext(httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/ws_1/mail-logs/msg_1/events", nil), "ws_1", "ak_1", []string{"mail_logs.read"})
	req = req.WithContext(chiCtx(req.Context(), "message_id", "msg_1"))
	rec := httptest.NewRecorder()
	h.listMailLogEvents(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestMailLogs_CompatibilityAlias(t *testing.T) {
	now := nowUTC()
	svc := &mockDeliverySvc{
		listMessagesFn: func(ctx context.Context, input deliveryapp.ListMessagesInput) (*deliveryapp.ListMessagesResult, error) {
			return &deliveryapp.ListMessagesResult{
				Messages: []domain.Message{
					{ID: "msg_1", WorkspaceID: "ws_1", Status: domain.MessageStatusDelivered, CreatedAt: now},
				},
			}, nil
		},
		getMessageFn: func(ctx context.Context, input deliveryapp.GetMessageInput) (*domain.Message, error) {
			return &domain.Message{ID: "msg_1", WorkspaceID: "ws_1", Status: domain.MessageStatusDelivered, CreatedAt: now}, nil
		},
	}

	// Test old /messages list endpoint
	h := &deliveryHTTP{svc: svc}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/ws_1/messages", nil)
	req = req.WithContext(chiCtx(req.Context(), "workspace_id", "ws_1"))
	rec := httptest.NewRecorder()
	h.listMessages(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 from alias, got %d: %s", rec.Code, rec.Body.String())
	}
}
