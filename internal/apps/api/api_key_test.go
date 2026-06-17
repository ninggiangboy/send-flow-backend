package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	identityapp "github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app"
)

// mockAuditRecorderIdentity implements identityapp.AuditRecorder for testing
type mockAuditRecorderIdentity struct {
	entries []identityapp.RecordAuditInput
}

func (m *mockAuditRecorderIdentity) Record(ctx context.Context, input identityapp.RecordAuditInput) error {
	m.entries = append(m.entries, input)
	return nil
}

func TestAPIKey_AuditRecording(t *testing.T) {
	auditRec := &mockAuditRecorderIdentity{}

	h := &transactionalHTTP{
		svc:           nil,
		auditRecorder: auditRec,
	}

	// Test recordAudit helper
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	h.recordAudit(req, identityapp.RecordAuditInput{
		WorkspaceID: "ws_1",
		ActorUserID: "ak_1",
		ActionType:  "test.action",
		TargetType:  "test",
		TargetID:    "test_1",
		OccurredAt:  nowUTC(),
	})

	if len(auditRec.entries) != 1 {
		t.Fatalf("expected 1 audit entry, got %d", len(auditRec.entries))
	}
	if auditRec.entries[0].ActionType != "test.action" {
		t.Fatalf("expected action test.action, got %s", auditRec.entries[0].ActionType)
	}
}

func TestAPIKey_ContextPropagation(t *testing.T) {
	// Test that apiKeyContext helper sets workspace_id correctly
	req := apiKeyContext(httptest.NewRequest(http.MethodGet, "/api/v1/test", nil), "ws_test", "ak_test", []string{"mail_logs.read"})

	wsID, _ := req.Context().Value(ctxAPIKeyWorkspaceID).(string)
	if wsID != "ws_test" {
		t.Fatalf("expected workspace ws_test, got %s", wsID)
	}
	keyID, _ := req.Context().Value(ctxAPIKeyID).(string)
	if keyID != "ak_test" {
		t.Fatalf("expected key ak_test, got %s", keyID)
	}
}
