package recordauditentry

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

type stubEntryWriteRepo struct {
	auditports.EntryWriteRepository
	appendFunc func(ctx context.Context, entry auditdomain.AuditEntry) error
}

func (s *stubEntryWriteRepo) Append(ctx context.Context, entry auditdomain.AuditEntry) error {
	return s.appendFunc(ctx, entry)
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func now() time.Time {
	return time.Date(2026, 6, 11, 10, 0, 0, 0, time.UTC)
}

func TestExecute_Success(t *testing.T) {
	var capturedEntry *auditdomain.AuditEntry
	payload := map[string]any{"email": "test@example.com", "password": "secret"}
	h := New(Options{
		EntriesWrite: &stubEntryWriteRepo{
			appendFunc: func(ctx context.Context, entry auditdomain.AuditEntry) error {
				capturedEntry = &entry
				return nil
			},
		},
		IDGen:  func() (string, error) { return "aud_1", nil },
		Logger: testLogger(),
	})

	err := h.Execute(context.Background(), Command{
		WorkspaceID:    "ws_1",
		ActorUserID:    "user_1",
		ActionType:     "user.login",
		TargetType:     "user",
		TargetID:       "user_1",
		RequestID:      "req_1",
		PayloadSummary: payload,
		OccurredAt:     now(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedEntry == nil {
		t.Fatal("expected entry to be appended")
	}
	if capturedEntry.ID != "aud_1" {
		t.Errorf("expected id aud_1, got %s", capturedEntry.ID)
	}
	if capturedEntry.WorkspaceID != "ws_1" {
		t.Errorf("expected workspace_id ws_1, got %s", capturedEntry.WorkspaceID)
	}
	if capturedEntry.ActorUserID != "user_1" {
		t.Errorf("expected actor_user_id user_1, got %s", capturedEntry.ActorUserID)
	}
	if capturedEntry.ActionType != "user.login" {
		t.Errorf("expected action_type user.login, got %s", capturedEntry.ActionType)
	}
	if capturedEntry.TargetType != "user" {
		t.Errorf("expected target_type user, got %s", capturedEntry.TargetType)
	}
	if capturedEntry.TargetID != "user_1" {
		t.Errorf("expected target_id user_1, got %s", capturedEntry.TargetID)
	}
	if capturedEntry.RequestID != "req_1" {
		t.Errorf("expected request_id req_1, got %s", capturedEntry.RequestID)
	}
	if !capturedEntry.OccurredAt.Equal(now()) {
		t.Errorf("expected occurred_at %v, got %v", now(), capturedEntry.OccurredAt)
	}
	if capturedEntry.PayloadSummary["password"] != "***REDACTED***" {
		t.Errorf("expected password to be redacted, got %v", capturedEntry.PayloadSummary["password"])
	}
	if capturedEntry.PayloadSummary["email"] != "test@example.com" {
		t.Errorf("expected email to be unchanged, got %v", capturedEntry.PayloadSummary["email"])
	}
}

func TestExecute_IDGenError(t *testing.T) {
	h := New(Options{
		EntriesWrite: &stubEntryWriteRepo{},
		IDGen:        func() (string, error) { return "", errors.New("idgen failure") },
		Logger:       testLogger(),
	})

	err := h.Execute(context.Background(), Command{
		WorkspaceID: "ws_1",
		ActorUserID: "user_1",
		ActionType:  "user.login",
		OccurredAt:  now(),
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "generate audit entry id: idgen failure" {
		t.Fatalf("expected idgen failure message, got %v", err)
	}
}

func TestExecute_AppendError(t *testing.T) {
	h := New(Options{
		EntriesWrite: &stubEntryWriteRepo{
			appendFunc: func(ctx context.Context, entry auditdomain.AuditEntry) error {
				return errors.New("db unavailable")
			},
		},
		IDGen:  func() (string, error) { return "aud_1", nil },
		Logger: testLogger(),
	})

	err := h.Execute(context.Background(), Command{
		WorkspaceID: "ws_1",
		ActorUserID: "user_1",
		ActionType:  "user.login",
		OccurredAt:  now(),
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestExecute_EntryValidationError(t *testing.T) {
	h := New(Options{
		EntriesWrite: &stubEntryWriteRepo{},
		IDGen:        func() (string, error) { return "aud_1", nil },
		Logger:       testLogger(),
	})

	err := h.Execute(context.Background(), Command{
		WorkspaceID: "",
		ActorUserID: "user_1",
		ActionType:  "user.login",
		OccurredAt:  now(),
	})
	if !errors.Is(err, auditdomain.ErrAuditEntryInvalid) {
		t.Fatalf("expected ErrAuditEntryInvalid, got %v", err)
	}
}
