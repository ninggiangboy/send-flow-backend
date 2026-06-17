package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	accessapp "github.com/ninggiangboy/send-flow/backend/internal/modules/access/app"
	deliveryapp "github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/app"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/domain"
	identityapp "github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app"
)

// ---- Mock delivery service ----

type mockDeliverySvc struct {
	listMessagesFn            func(ctx context.Context, input deliveryapp.ListMessagesInput) (*deliveryapp.ListMessagesResult, error)
	getMessageFn              func(ctx context.Context, input deliveryapp.GetMessageInput) (*domain.Message, error)
	acceptTransactionalSendFn func(ctx context.Context, input deliveryapp.AcceptTransactionalSendInput) (*deliveryapp.AcceptTransactionalSendResult, error)
	getTransactionalMessageFn func(ctx context.Context, input deliveryapp.GetTransactionalMessageInput) (*deliveryapp.GetTransactionalMessageResult, error)
	listMessageEventsFn       func(ctx context.Context, input deliveryapp.ListMessageEventsInput) (*deliveryapp.ListMessageEventsResult, error)
	listRequestMessagesFn     func(ctx context.Context, input deliveryapp.ListRequestMessagesInput) (*deliveryapp.ListRequestMessagesResult, error)
	listAttemptsFn            func(ctx context.Context, input deliveryapp.ListAttemptsInput) ([]domain.DeliveryAttempt, error)
}

func (m *mockDeliverySvc) ListMessages(ctx context.Context, input deliveryapp.ListMessagesInput) (*deliveryapp.ListMessagesResult, error) {
	return m.listMessagesFn(ctx, input)
}

func (m *mockDeliverySvc) GetMessage(ctx context.Context, input deliveryapp.GetMessageInput) (*domain.Message, error) {
	return m.getMessageFn(ctx, input)
}

func (m *mockDeliverySvc) AcceptTransactionalSend(ctx context.Context, input deliveryapp.AcceptTransactionalSendInput) (*deliveryapp.AcceptTransactionalSendResult, error) {
	return m.acceptTransactionalSendFn(ctx, input)
}

func (m *mockDeliverySvc) GetTransactionalMessage(ctx context.Context, input deliveryapp.GetTransactionalMessageInput) (*deliveryapp.GetTransactionalMessageResult, error) {
	return m.getTransactionalMessageFn(ctx, input)
}

func (m *mockDeliverySvc) ListMessageEvents(ctx context.Context, input deliveryapp.ListMessageEventsInput) (*deliveryapp.ListMessageEventsResult, error) {
	return m.listMessageEventsFn(ctx, input)
}

func (m *mockDeliverySvc) ListRequestMessages(ctx context.Context, input deliveryapp.ListRequestMessagesInput) (*deliveryapp.ListRequestMessagesResult, error) {
	return m.listRequestMessagesFn(ctx, input)
}

func (m *mockDeliverySvc) ListAttempts(ctx context.Context, input deliveryapp.ListAttemptsInput) ([]domain.DeliveryAttempt, error) {
	return m.listAttemptsFn(ctx, input)
}

// ---- Mock access service (API key auth) ----

type mockAccessSvc struct {
	authenticateAPIKeyFn func(ctx context.Context, input accessapp.AuthenticateAPIKeyInput) (*accessapp.AuthenticatedAPIKey, error)
}

func (m *mockAccessSvc) AuthenticateAPIKey(ctx context.Context, input accessapp.AuthenticateAPIKeyInput) (*accessapp.AuthenticatedAPIKey, error) {
	if m.authenticateAPIKeyFn == nil {
		return &accessapp.AuthenticatedAPIKey{
			APIKeyID:    "test_key_id",
			WorkspaceID: "ws_1",
			KeyPrefix:   "test_",
			Scopes:      []string{"transactional.send", "transactional.read", "mail_logs.read"},
		}, nil
	}
	return m.authenticateAPIKeyFn(ctx, input)
}

// ---- Mock audit recorder ----

type mockAuditRecorder struct {
	entries []identityapp.RecordAuditInput
}

func (m *mockAuditRecorder) Record(ctx context.Context, input identityapp.RecordAuditInput) error {
	m.entries = append(m.entries, input)
	return nil
}

// ---- Test helpers ----

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func apiKeyContext(req *http.Request, workspaceID, apiKeyID string, scopes []string) *http.Request {
	ctx := context.WithValue(req.Context(), ctxAPIKeyWorkspaceID, workspaceID)
	ctx = context.WithValue(ctx, ctxAPIKeyID, apiKeyID)
	ctx = context.WithValue(ctx, ctxAPIKeyScopes, scopes)
	ctx = context.WithValue(ctx, ctxAPIKeyPrefix, "test_")
	return req.WithContext(ctx)
}

func nowUTC() time.Time {
	return time.Date(2026, 6, 18, 10, 0, 0, 0, time.UTC)
}

func encodeJSONBody(t *testing.T, v any) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(v); err != nil {
		t.Fatalf("encode body: %v", err)
	}
	return &buf
}

func chiCtx(ctx context.Context, params ...string) context.Context {
	rctx := chi.NewRouteContext()
	for i := 0; i < len(params)-1; i += 2 {
		rctx.URLParams.Add(params[i], params[i+1])
	}
	return context.WithValue(ctx, chi.RouteCtxKey, rctx)
}
