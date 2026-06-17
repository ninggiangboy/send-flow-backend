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

func TestDeliveryListMessages(t *testing.T) {
	now := nowUTC()
	svc := &mockDeliverySvc{
		listMessagesFn: func(ctx context.Context, input deliveryapp.ListMessagesInput) (*deliveryapp.ListMessagesResult, error) {
			if input.WorkspaceID != "ws_1" {
				t.Fatalf("expected workspace ws_1, got %s", input.WorkspaceID)
			}
			return &deliveryapp.ListMessagesResult{
				Messages: []domain.Message{
					{ID: "msg_1", WorkspaceID: "ws_1", Status: domain.MessageStatusDelivered, Subject: "Hello", MessageType: domain.MessageTypeTransactional, CreatedAt: now},
				},
			}, nil
		},
	}
	h := &deliveryHTTP{svc: svc}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/ws_1/messages", nil)
	req = req.WithContext(chiCtx(req.Context(), "workspace_id", "ws_1"))
	rec := httptest.NewRecorder()
	h.listMessages(rec, req)

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

func TestDeliveryGetMessage(t *testing.T) {
	now := nowUTC()
	svc := &mockDeliverySvc{
		getMessageFn: func(ctx context.Context, input deliveryapp.GetMessageInput) (*domain.Message, error) {
			return &domain.Message{
				ID:                       "msg_1",
				WorkspaceID:              "ws_1",
				RecipientEmailNormalized: "a@example.com",
				Status:                   domain.MessageStatusDelivered,
				Subject:                  "Test",
				MessageType:              domain.MessageTypeTransactional,
				CreatedAt:                now,
			}, nil
		},
	}
	h := &deliveryHTTP{svc: svc}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/ws_1/messages/msg_1", nil)
	req = req.WithContext(chiCtx(req.Context(), "workspace_id", "ws_1", "message_id", "msg_1"))
	rec := httptest.NewRecorder()
	h.getMessage(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestDeliveryGetMessage_NotFound(t *testing.T) {
	svc := &mockDeliverySvc{
		getMessageFn: func(ctx context.Context, input deliveryapp.GetMessageInput) (*domain.Message, error) {
			return nil, domain.ErrMessageNotFound
		},
	}
	h := &deliveryHTTP{svc: svc}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces/ws_1/messages/msg_notfound", nil)
	req = req.WithContext(chiCtx(req.Context(), "workspace_id", "ws_1", "message_id", "msg_notfound"))
	rec := httptest.NewRecorder()
	h.getMessage(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}
