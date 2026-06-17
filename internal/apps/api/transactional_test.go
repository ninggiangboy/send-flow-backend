package api

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"context"

	deliveryapp "github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/app"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/domain"
)

func TestTransactionalSend_JSON_TemplateMode_Success(t *testing.T) {
	svc := &mockDeliverySvc{
		acceptTransactionalSendFn: func(ctx context.Context, input deliveryapp.AcceptTransactionalSendInput) (*deliveryapp.AcceptTransactionalSendResult, error) {
			if input.Mode != domain.MessageModeTemplate {
				t.Fatalf("expected mode template, got %s", input.Mode)
			}
			return &deliveryapp.AcceptTransactionalSendResult{
				RequestID:  "txreq_1",
				MessageIDs: []string{"msg_1"},
				Status:     domain.TxRequestStatusAccepted,
				AcceptedAt: time.Now().UTC(),
			}, nil
		},
	}
	h := &transactionalHTTP{svc: svc}

	body := map[string]any{
		"mode":             "template",
		"to":               []map[string]any{{"email": "alice@example.com", "name": "Alice"}},
		"sender_domain_id": "sd_123",
		"template_id":      "tmpl_welcome",
		"template_data":    map[string]any{"first_name": "Alice"},
	}
	req := apiKeyContext(httptest.NewRequest(http.MethodPost, "/", encodeJSONBody(t, body)), "ws_1", "ak_1", []string{"transactional.send"})
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.send(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	data := resp["data"].(map[string]any)
	if data["request_id"] != "txreq_1" {
		t.Fatalf("expected request_id txreq_1, got %v", data["request_id"])
	}
	if data["status"] != domain.TxRequestStatusAccepted {
		t.Fatalf("expected status accepted, got %v", data["status"])
	}
}

func TestTransactionalSend_JSON_RawMode_Success(t *testing.T) {
	svc := &mockDeliverySvc{
		acceptTransactionalSendFn: func(ctx context.Context, input deliveryapp.AcceptTransactionalSendInput) (*deliveryapp.AcceptTransactionalSendResult, error) {
			if input.Mode != domain.MessageModeRaw {
				t.Fatalf("expected mode raw, got %s", input.Mode)
			}
			return &deliveryapp.AcceptTransactionalSendResult{
				RequestID:  "txreq_2",
				MessageIDs: []string{"msg_2"},
				Status:     domain.TxRequestStatusAccepted,
				AcceptedAt: time.Now().UTC(),
			}, nil
		},
	}
	h := &transactionalHTTP{svc: svc}

	body := map[string]any{
		"mode":             "raw",
		"to":               []map[string]any{{"email": "bob@example.com"}},
		"sender_domain_id": "sd_123",
		"subject":          "Test",
		"text_body":        "Hello",
		"html_body":        "<p>Hello</p>",
	}
	req := apiKeyContext(httptest.NewRequest(http.MethodPost, "/", encodeJSONBody(t, body)), "ws_1", "ak_1", []string{"transactional.send"})
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.send(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestTransactionalSend_Multipart_RawWithAttachments(t *testing.T) {
	svc := &mockDeliverySvc{
		acceptTransactionalSendFn: func(ctx context.Context, input deliveryapp.AcceptTransactionalSendInput) (*deliveryapp.AcceptTransactionalSendResult, error) {
			if len(input.Attachments) != 1 {
				t.Fatalf("expected 1 attachment, got %d", len(input.Attachments))
			}
			return &deliveryapp.AcceptTransactionalSendResult{
				RequestID:  "txreq_3",
				MessageIDs: []string{"msg_3"},
				Status:     domain.TxRequestStatusAccepted,
				AcceptedAt: time.Now().UTC(),
			}, nil
		},
	}
	h := &transactionalHTTP{svc: svc}

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	w.WriteField("mode", "raw")
	w.WriteField("to", `[{"email":"a@example.com"}]`)
	w.WriteField("sender_domain_id", "sd_123")
	w.WriteField("subject", "Test")
	w.WriteField("text_body", "Hello")
	w.WriteField("html_body", "<p>Hello</p>")
	part, _ := w.CreateFormFile("attachments", "test.txt")
	part.Write([]byte("hello world"))
	w.Close()

	req := apiKeyContext(httptest.NewRequest(http.MethodPost, "/", &buf), "ws_1", "ak_1", []string{"transactional.send"})
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec := httptest.NewRecorder()
	h.send(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestTransactionalSend_InvalidContentType_415(t *testing.T) {
	h := &transactionalHTTP{}
	req := apiKeyContext(httptest.NewRequest(http.MethodPost, "/", strings.NewReader("plain")), "ws_1", "ak_1", []string{"transactional.send"})
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()
	h.send(rec, req)

	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("expected 415, got %d", rec.Code)
	}
}

func TestTransactionalSend_MissingWorkspace_401(t *testing.T) {
	h := &transactionalHTTP{}
	// No workspace context
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.send(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestTransactionalGetMessage_Success(t *testing.T) {
	now := nowUTC()
	svc := &mockDeliverySvc{
		getTransactionalMessageFn: func(ctx context.Context, input deliveryapp.GetTransactionalMessageInput) (*deliveryapp.GetTransactionalMessageResult, error) {
			return &deliveryapp.GetTransactionalMessageResult{
				MessageID:         "msg_1",
				Status:            domain.MessageStatusDelivered,
				Provider:          "ses",
				ProviderMessageID: "ses_123",
				LastUpdatedAt:     &now,
			}, nil
		},
	}
	h := &transactionalHTTP{svc: svc}
	req := apiKeyContext(httptest.NewRequest(http.MethodGet, "/api/v1/transactional/messages/msg_1", nil), "ws_1", "ak_1", []string{"transactional.read"})
	rec := httptest.NewRecorder()
	h.getMessage(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestTransactionalGetMessage_NotFound(t *testing.T) {
	svc := &mockDeliverySvc{
		getTransactionalMessageFn: func(ctx context.Context, input deliveryapp.GetTransactionalMessageInput) (*deliveryapp.GetTransactionalMessageResult, error) {
			return nil, domain.ErrMessageNotFound
		},
	}
	h := &transactionalHTTP{svc: svc}
	req := apiKeyContext(httptest.NewRequest(http.MethodGet, "/api/v1/transactional/messages/msg_notfound", nil), "ws_1", "ak_1", []string{"transactional.read"})
	rec := httptest.NewRecorder()
	h.getMessage(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestTransactionalListMessageEvents(t *testing.T) {
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
	h := &transactionalHTTP{svc: svc}
	req := apiKeyContext(httptest.NewRequest(http.MethodGet, "/api/v1/transactional/messages/msg_1/events", nil), "ws_1", "ak_1", []string{"mail_logs.read"})
	req = req.WithContext(chiCtx(req.Context(), "message_id", "msg_1"))
	rec := httptest.NewRecorder()
	h.listMessageEvents(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	data := resp["data"].(map[string]any)
	events := data["events"].([]any)
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
}

func TestTransactionalListRequestMessages(t *testing.T) {
	now := nowUTC()
	svc := &mockDeliverySvc{
		listRequestMessagesFn: func(ctx context.Context, input deliveryapp.ListRequestMessagesInput) (*deliveryapp.ListRequestMessagesResult, error) {
			return &deliveryapp.ListRequestMessagesResult{
				Messages: []domain.Message{
					{ID: "msg_1", WorkspaceID: "ws_1", TransactionalRequestID: "txreq_1", RecipientEmailNormalized: "a@example.com", Status: domain.MessageStatusQueued, MessageType: domain.MessageTypeTransactional, CreatedAt: now},
					{ID: "msg_2", WorkspaceID: "ws_1", TransactionalRequestID: "txreq_1", RecipientEmailNormalized: "b@example.com", Status: domain.MessageStatusQueued, MessageType: domain.MessageTypeTransactional, CreatedAt: now},
				},
			}, nil
		},
	}
	h := &transactionalHTTP{svc: svc}
	req := apiKeyContext(httptest.NewRequest(http.MethodGet, "/api/v1/transactional/requests/txreq_1/messages", nil), "ws_1", "ak_1", []string{"mail_logs.read"})
	req = req.WithContext(chiCtx(req.Context(), "request_id", "txreq_1"))
	rec := httptest.NewRecorder()
	h.listRequestMessages(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}
