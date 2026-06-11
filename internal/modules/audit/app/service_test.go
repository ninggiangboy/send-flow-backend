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

func TestRecordAuditEntry_Success(t *testing.T) {
	var capturedEntry *auditdomain.AuditEntry
	payload := map[string]any{"email": "test@example.com", "password": "secret"}
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

func TestRecordAuditEntry_IDGenError(t *testing.T) {
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
	if err.Error() != "generate audit entry id: idgen failure" {
		t.Fatalf("expected idgen failure message, got %v", err)
	}
}

func TestRecordAuditEntry_AppendError(t *testing.T) {
	svc := NewService(Options{
		EntriesRead: &stubEntryReadRepo{},
		EntriesWrite: &stubEntryWriteRepo{
			appendFunc: func(ctx context.Context, entry auditdomain.AuditEntry) error {
				return errors.New("db unavailable")
			},
		},
		PermChecker: &stubPermChecker{},
		IDGen:       func() (string, error) { return "aud_1", nil },
		Logger:      testLogger(),
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

func TestListAuditEntries_Success(t *testing.T) {
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
	if entries[0].ID != "aud_1" {
		t.Errorf("expected first entry aud_1, got %s", entries[0].ID)
	}
	if entries[1].ID != "aud_2" {
		t.Errorf("expected second entry aud_2, got %s", entries[1].ID)
	}
	if next != "" {
		t.Errorf("expected empty cursor, got %s", next)
	}
}

func TestListAuditEntries_PermissionDenied(t *testing.T) {
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

func TestListAuditEntries_PermissionCheckerError(t *testing.T) {
	svc := NewService(Options{
		EntriesRead: &stubEntryReadRepo{},
		PermChecker: &stubPermChecker{
			requirePermissionFunc: func(ctx context.Context, workspaceID, userID, permission string) error {
				return errors.New("unexpected permission error")
			},
		},
		Logger: testLogger(),
	})

	_, _, err := svc.ListAuditEntries(context.Background(), ListAuditEntriesInput{
		WorkspaceID: "ws_1",
		UserID:      "user_1",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestListAuditEntries_Empty(t *testing.T) {
	svc := NewService(Options{
		EntriesRead: &stubEntryReadRepo{
			listFunc: func(ctx context.Context, filter auditdomain.AuditFilter) ([]auditdomain.AuditEntry, string, error) {
				return []auditdomain.AuditEntry{}, "", nil
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
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected empty entries, got %d", len(entries))
	}
	if next != "" {
		t.Errorf("expected empty cursor, got %s", next)
	}
}

func TestListAuditEntries_FilterValidationFromAfterTo(t *testing.T) {
	from := now().Add(time.Hour)
	to := now()
	svc := NewService(Options{
		EntriesRead: &stubEntryReadRepo{},
		PermChecker: &stubPermChecker{},
		Logger:      testLogger(),
	})

	_, _, err := svc.ListAuditEntries(context.Background(), ListAuditEntriesInput{
		WorkspaceID: "ws_1",
		UserID:      "user_1",
		From:        &from,
		To:          &to,
	})
	if !errors.Is(err, auditdomain.ErrAuditFilterInvalid) {
		t.Fatalf("expected ErrAuditFilterInvalid, got %v", err)
	}
}

func TestListAuditEntries_FilterValidationFromEqualsTo(t *testing.T) {
	ts := now()
	svc := NewService(Options{
		EntriesRead: &stubEntryReadRepo{
			listFunc: func(ctx context.Context, filter auditdomain.AuditFilter) ([]auditdomain.AuditEntry, string, error) {
				return []auditdomain.AuditEntry{}, "", nil
			},
		},
		PermChecker: &stubPermChecker{
			requirePermissionFunc: func(ctx context.Context, workspaceID, userID, permission string) error {
				return nil
			},
		},
		Logger: testLogger(),
	})

	_, _, err := svc.ListAuditEntries(context.Background(), ListAuditEntriesInput{
		WorkspaceID: "ws_1",
		UserID:      "user_1",
		From:        &ts,
		To:          &ts,
	})
	if err != nil {
		t.Fatalf("unexpected error when from equals to: %v", err)
	}
}

func TestListAuditEntries_LimitClamping(t *testing.T) {
	svc := NewService(Options{
		EntriesRead: &stubEntryReadRepo{},
		PermChecker: &stubPermChecker{
			requirePermissionFunc: func(ctx context.Context, workspaceID, userID, permission string) error {
				return nil
			},
		},
		Logger: testLogger(),
	})

	_, _, err := svc.ListAuditEntries(context.Background(), ListAuditEntriesInput{
		WorkspaceID: "ws_1",
		UserID:      "user_1",
		Limit:       200,
	})
	if err == nil {
		t.Fatal("expected error for limit > 100")
	}
	if !errors.Is(err, auditdomain.ErrAuditFilterInvalid) {
		t.Fatalf("expected ErrAuditFilterInvalid, got %v", err)
	}
}

func TestListAuditEntries_DefaultLimit(t *testing.T) {
	var capturedFilter auditdomain.AuditFilter
	svc := NewService(Options{
		EntriesRead: &stubEntryReadRepo{
			listFunc: func(ctx context.Context, filter auditdomain.AuditFilter) ([]auditdomain.AuditEntry, string, error) {
				capturedFilter = filter
				return []auditdomain.AuditEntry{}, "", nil
			},
		},
		PermChecker: &stubPermChecker{
			requirePermissionFunc: func(ctx context.Context, workspaceID, userID, permission string) error {
				return nil
			},
		},
		Logger: testLogger(),
	})

	_, _, err := svc.ListAuditEntries(context.Background(), ListAuditEntriesInput{
		WorkspaceID: "ws_1",
		UserID:      "user_1",
		Limit:       0,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedFilter.Limit != 50 {
		t.Errorf("expected default limit 50, got %d", capturedFilter.Limit)
	}
}

func TestListAuditEntries_NegativeLimit(t *testing.T) {
	var capturedFilter auditdomain.AuditFilter
	svc := NewService(Options{
		EntriesRead: &stubEntryReadRepo{
			listFunc: func(ctx context.Context, filter auditdomain.AuditFilter) ([]auditdomain.AuditEntry, string, error) {
				capturedFilter = filter
				return []auditdomain.AuditEntry{}, "", nil
			},
		},
		PermChecker: &stubPermChecker{
			requirePermissionFunc: func(ctx context.Context, workspaceID, userID, permission string) error {
				return nil
			},
		},
		Logger: testLogger(),
	})

	_, _, err := svc.ListAuditEntries(context.Background(), ListAuditEntriesInput{
		WorkspaceID: "ws_1",
		UserID:      "user_1",
		Limit:       -5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedFilter.Limit != 50 {
		t.Errorf("expected default limit 50 for negative input, got %d", capturedFilter.Limit)
	}
}

func TestListAuditEntries_RepositoryError(t *testing.T) {
	svc := NewService(Options{
		EntriesRead: &stubEntryReadRepo{
			listFunc: func(ctx context.Context, filter auditdomain.AuditFilter) ([]auditdomain.AuditEntry, string, error) {
				return nil, "", errors.New("db connection lost")
			},
		},
		PermChecker: &stubPermChecker{
			requirePermissionFunc: func(ctx context.Context, workspaceID, userID, permission string) error {
				return nil
			},
		},
		Logger: testLogger(),
	})

	_, _, err := svc.ListAuditEntries(context.Background(), ListAuditEntriesInput{
		WorkspaceID: "ws_1",
		UserID:      "user_1",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestListAuditEntries_FilterPassthrough(t *testing.T) {
	var capturedFilter auditdomain.AuditFilter
	from := now()
	to := now().Add(time.Hour)
	svc := NewService(Options{
		EntriesRead: &stubEntryReadRepo{
			listFunc: func(ctx context.Context, filter auditdomain.AuditFilter) ([]auditdomain.AuditEntry, string, error) {
				capturedFilter = filter
				return []auditdomain.AuditEntry{}, "", nil
			},
		},
		PermChecker: &stubPermChecker{
			requirePermissionFunc: func(ctx context.Context, workspaceID, userID, permission string) error {
				return nil
			},
		},
		Logger: testLogger(),
	})

	_, _, err := svc.ListAuditEntries(context.Background(), ListAuditEntriesInput{
		WorkspaceID: "ws_1",
		UserID:      "user_1",
		ActorUserID: "actor_1",
		ActionType:  "user.login",
		TargetType:  "user",
		TargetID:    "user_1",
		From:        &from,
		To:          &to,
		Limit:       25,
		Cursor:      "cursor_abc",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedFilter.WorkspaceID != "ws_1" {
		t.Errorf("expected workspace_id ws_1, got %s", capturedFilter.WorkspaceID)
	}
	if capturedFilter.ActorUserID != "actor_1" {
		t.Errorf("expected actor_user_id actor_1, got %s", capturedFilter.ActorUserID)
	}
	if capturedFilter.ActionType != "user.login" {
		t.Errorf("expected action_type user.login, got %s", capturedFilter.ActionType)
	}
	if capturedFilter.TargetType != "user" {
		t.Errorf("expected target_type user, got %s", capturedFilter.TargetType)
	}
	if capturedFilter.TargetID != "user_1" {
		t.Errorf("expected target_id user_1, got %s", capturedFilter.TargetID)
	}
	if !capturedFilter.From.Equal(from) {
		t.Errorf("expected from %v, got %v", from, capturedFilter.From)
	}
	if !capturedFilter.To.Equal(to) {
		t.Errorf("expected to %v, got %v", to, capturedFilter.To)
	}
	if capturedFilter.Limit != 25 {
		t.Errorf("expected limit 25, got %d", capturedFilter.Limit)
	}
	if capturedFilter.Cursor != "cursor_abc" {
		t.Errorf("expected cursor cursor_abc, got %s", capturedFilter.Cursor)
	}
}

func TestListAuditEntries_ValidLimit100(t *testing.T) {
	svc := NewService(Options{
		EntriesRead: &stubEntryReadRepo{
			listFunc: func(ctx context.Context, filter auditdomain.AuditFilter) ([]auditdomain.AuditEntry, string, error) {
				if filter.Limit != 100 {
					t.Errorf("expected limit 100, got %d", filter.Limit)
				}
				return []auditdomain.AuditEntry{}, "", nil
			},
		},
		PermChecker: &stubPermChecker{
			requirePermissionFunc: func(ctx context.Context, workspaceID, userID, permission string) error {
				return nil
			},
		},
		Logger: testLogger(),
	})

	_, _, err := svc.ListAuditEntries(context.Background(), ListAuditEntriesInput{
		WorkspaceID: "ws_1",
		UserID:      "user_1",
		Limit:       100,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRecordAuditEntry_EntryValidationError(t *testing.T) {
	svc := NewService(Options{
		EntriesRead:  &stubEntryReadRepo{},
		EntriesWrite: &stubEntryWriteRepo{},
		PermChecker:  &stubPermChecker{},
		IDGen:        func() (string, error) { return "aud_1", nil },
		Logger:       testLogger(),
	})

	err := svc.RecordAuditEntry(context.Background(), RecordAuditEntryInput{
		WorkspaceID: "",
		ActorUserID: "user_1",
		ActionType:  "user.login",
		OccurredAt:  now(),
	})
	if !errors.Is(err, auditdomain.ErrAuditEntryInvalid) {
		t.Fatalf("expected ErrAuditEntryInvalid, got %v", err)
	}
}

func TestListAuditEntries_PermissionCheckCalled(t *testing.T) {
	permChecked := false
	svc := NewService(Options{
		EntriesRead: &stubEntryReadRepo{
			listFunc: func(ctx context.Context, filter auditdomain.AuditFilter) ([]auditdomain.AuditEntry, string, error) {
				return []auditdomain.AuditEntry{}, "", nil
			},
		},
		PermChecker: &stubPermChecker{
			requirePermissionFunc: func(ctx context.Context, workspaceID, userID, permission string) error {
				permChecked = true
				return nil
			},
		},
		Logger: testLogger(),
	})

	_, _, err := svc.ListAuditEntries(context.Background(), ListAuditEntriesInput{
		WorkspaceID: "ws_1",
		UserID:      "user_1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !permChecked {
		t.Error("expected permission check to be called")
	}
}
