package entry

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	auditdomain "github.com/ninggiangboy/send-flow/backend/internal/modules/audit/domain"
	auditports "github.com/ninggiangboy/send-flow/backend/internal/modules/audit/ports"
)

func auditTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type auditEntryWriteStub struct {
	auditports.EntryWriteRepository
	appendFunc func(context.Context, auditdomain.AuditEntry) error
}

func (s *auditEntryWriteStub) Append(ctx context.Context, entry auditdomain.AuditEntry) error {
	return s.appendFunc(ctx, entry)
}

func auditNow() time.Time {
	return time.Date(2026, 6, 11, 10, 0, 0, 0, time.UTC)
}

func TestRecordHandlerExecuteSuccess(t *testing.T) {
	var captured *auditdomain.AuditEntry
	h := NewRecordHandler(struct {
		EntriesWrite auditports.EntryWriteRepository
		IDGen        func() (string, error)
		Logger       *slog.Logger
	}{
		EntriesWrite: &auditEntryWriteStub{appendFunc: func(ctx context.Context, entry auditdomain.AuditEntry) error { captured = &entry; return nil }},
		IDGen:        func() (string, error) { return "aud_1", nil },
		Logger:       auditTestLogger(),
	})

	err := h.Execute(context.Background(), RecordInput{
		WorkspaceID:    "ws_1",
		ActorUserID:    "user_1",
		ActionType:     "user.login",
		TargetType:     "user",
		TargetID:       "user_1",
		RequestID:      "req_1",
		PayloadSummary: map[string]any{"email": "test@example.com", "password": "secret"},
		OccurredAt:     auditNow(),
	})
	if err != nil || captured == nil || captured.PayloadSummary["password"] != "***REDACTED***" {
		t.Fatalf("unexpected result: captured=%+v err=%v", captured, err)
	}
}

func TestRecordHandlerExecuteIDGenError(t *testing.T) {
	h := NewRecordHandler(struct {
		EntriesWrite auditports.EntryWriteRepository
		IDGen        func() (string, error)
		Logger       *slog.Logger
	}{
		EntriesWrite: &auditEntryWriteStub{appendFunc: func(context.Context, auditdomain.AuditEntry) error { return nil }},
		IDGen:        func() (string, error) { return "", errors.New("idgen failure") },
		Logger:       auditTestLogger(),
	})

	err := h.Execute(context.Background(), RecordInput{WorkspaceID: "ws_1", ActorUserID: "user_1", ActionType: "user.login", OccurredAt: auditNow()})
	if err == nil || err.Error() != "generate audit entry id: idgen failure" {
		t.Fatalf("unexpected error: %v", err)
	}
}
