package app

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

type stubEntryReadRepo struct {
	auditports.EntryReadRepository
	listFunc func(ctx context.Context, filter auditdomain.AuditFilter) ([]auditdomain.AuditEntry, string, error)
}

func (s *stubEntryReadRepo) List(ctx context.Context, filter auditdomain.AuditFilter) ([]auditdomain.AuditEntry, string, error) {
	return s.listFunc(ctx, filter)
}

type stubPermChecker struct {
	auditports.PermissionChecker
	requirePermissionFunc func(ctx context.Context, workspaceID, userID, permission string) error
}

func (s *stubPermChecker) RequirePermission(ctx context.Context, workspaceID, userID, permission string) error {
	return s.requirePermissionFunc(ctx, workspaceID, userID, permission)
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func now() time.Time {
	return time.Date(2026, 6, 11, 10, 0, 0, 0, time.UTC)
}

func TestService_RecordAuditEntry(t *testing.T) {
	var capturedEntry *auditdomain.AuditEntry
	svc := NewService(Options{
		EntriesRead: &stubEntryReadRepo{},
		EntriesWrite: &stubEntryWriteRepo{
			appendFunc: func(ctx context.Context, entry auditdomain.AuditEntry) error {
				capturedEntry = &entry
				return nil
			},
		},
		PermChecker: &stubPermChecker{},
		IDGen:       func() (string, error) { return "aud_1", nil },
		Logger:      testLogger(),
	})

	err := svc.RecordAuditEntry(context.Background(), RecordAuditEntryInput{
		WorkspaceID:    "ws_1",
		ActorUserID:    "user_1",
		ActionType:     "user.login",
		TargetType:     "user",
		TargetID:       "user_1",
		RequestID:      "req_1",
		PayloadSummary: map[string]any{"email": "test@example.com"},
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
}

func TestService_RecordAuditEntry_IDGenError(t *testing.T) {
	svc := NewService(Options{
		EntriesRead:  &stubEntryReadRepo{},
		EntriesWrite: &stubEntryWriteRepo{},
		PermChecker:  &stubPermChecker{},
		IDGen:        func() (string, error) { return "", errors.New("idgen failure") },
		Logger:       testLogger(),
	})

	err := svc.RecordAuditEntry(context.Background(), RecordAuditEntryInput{
		WorkspaceID: "ws_1",
		ActorUserID: "user_1",
		ActionType:  "user.login",
		OccurredAt:  now(),
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestService_ListAuditEntries(t *testing.T) {
	expectedEntries := []auditdomain.AuditEntry{
		{ID: "aud_1", WorkspaceID: "ws_1", ActionType: "user.login", OccurredAt: now()},
		{ID: "aud_2", WorkspaceID: "ws_1", ActionType: "user.logout", OccurredAt: now().Add(time.Minute)},
	}
	svc := NewService(Options{
		EntriesRead: &stubEntryReadRepo{
			listFunc: func(ctx context.Context, filter auditdomain.AuditFilter) ([]auditdomain.AuditEntry, string, error) {
				return expectedEntries, "", nil
			},
		},
		PermChecker: &stubPermChecker{
			requirePermissionFunc: func(ctx context.Context, workspaceID, userID, permission string) error {
				return nil
			},
		},
		Logger: testLogger(),
	})

	entries, next, err := svc.ListAuditEntries(context.Background(), ListAuditEntriesInput{
		WorkspaceID: "ws_1",
		UserID:      "user_1",
		Limit:       10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if next != "" {
		t.Errorf("expected empty cursor, got %s", next)
	}
}

func TestService_ListAuditEntries_PermissionDenied(t *testing.T) {
	svc := NewService(Options{
		EntriesRead: &stubEntryReadRepo{},
		PermChecker: &stubPermChecker{
			requirePermissionFunc: func(ctx context.Context, workspaceID, userID, permission string) error {
				return auditdomain.ErrAuditReadDenied
			},
		},
		Logger: testLogger(),
	})

	_, _, err := svc.ListAuditEntries(context.Background(), ListAuditEntriesInput{
		WorkspaceID: "ws_1",
		UserID:      "user_1",
	})
	if !errors.Is(err, auditdomain.ErrAuditReadDenied) {
		t.Fatalf("expected ErrAuditReadDenied, got %v", err)
	}
}
