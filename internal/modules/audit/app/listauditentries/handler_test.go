package listauditentries

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

func TestExecute_Success(t *testing.T) {
	expectedEntries := []auditdomain.AuditEntry{
		{ID: "aud_1", WorkspaceID: "ws_1", ActionType: "user.login", OccurredAt: now()},
		{ID: "aud_2", WorkspaceID: "ws_1", ActionType: "user.logout", OccurredAt: now().Add(time.Minute)},
	}
	h := New(Options{
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

	entries, next, err := h.Execute(context.Background(), Command{
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

func TestExecute_PermissionDenied(t *testing.T) {
	h := New(Options{
		EntriesRead: &stubEntryReadRepo{},
		PermChecker: &stubPermChecker{
			requirePermissionFunc: func(ctx context.Context, workspaceID, userID, permission string) error {
				return auditdomain.ErrAuditReadDenied
			},
		},
		Logger: testLogger(),
	})

	_, _, err := h.Execute(context.Background(), Command{
		WorkspaceID: "ws_1",
		UserID:      "user_1",
	})
	if !errors.Is(err, auditdomain.ErrAuditReadDenied) {
		t.Fatalf("expected ErrAuditReadDenied, got %v", err)
	}
}

func TestExecute_PermissionCheckerError(t *testing.T) {
	h := New(Options{
		EntriesRead: &stubEntryReadRepo{},
		PermChecker: &stubPermChecker{
			requirePermissionFunc: func(ctx context.Context, workspaceID, userID, permission string) error {
				return errors.New("unexpected permission error")
			},
		},
		Logger: testLogger(),
	})

	_, _, err := h.Execute(context.Background(), Command{
		WorkspaceID: "ws_1",
		UserID:      "user_1",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestExecute_Empty(t *testing.T) {
	h := New(Options{
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

	entries, next, err := h.Execute(context.Background(), Command{
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

func TestExecute_FilterValidationFromAfterTo(t *testing.T) {
	from := now().Add(time.Hour)
	to := now()
	h := New(Options{
		EntriesRead: &stubEntryReadRepo{},
		PermChecker: &stubPermChecker{},
		Logger:      testLogger(),
	})

	_, _, err := h.Execute(context.Background(), Command{
		WorkspaceID: "ws_1",
		UserID:      "user_1",
		From:        &from,
		To:          &to,
	})
	if !errors.Is(err, auditdomain.ErrAuditFilterInvalid) {
		t.Fatalf("expected ErrAuditFilterInvalid, got %v", err)
	}
}

func TestExecute_FilterValidationFromEqualsTo(t *testing.T) {
	ts := now()
	h := New(Options{
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

	_, _, err := h.Execute(context.Background(), Command{
		WorkspaceID: "ws_1",
		UserID:      "user_1",
		From:        &ts,
		To:          &ts,
	})
	if err != nil {
		t.Fatalf("unexpected error when from equals to: %v", err)
	}
}

func TestExecute_LimitClamping(t *testing.T) {
	h := New(Options{
		EntriesRead: &stubEntryReadRepo{},
		PermChecker: &stubPermChecker{
			requirePermissionFunc: func(ctx context.Context, workspaceID, userID, permission string) error {
				return nil
			},
		},
		Logger: testLogger(),
	})

	_, _, err := h.Execute(context.Background(), Command{
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

func TestExecute_DefaultLimit(t *testing.T) {
	var capturedFilter auditdomain.AuditFilter
	h := New(Options{
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

	_, _, err := h.Execute(context.Background(), Command{
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

func TestExecute_NegativeLimit(t *testing.T) {
	var capturedFilter auditdomain.AuditFilter
	h := New(Options{
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

	_, _, err := h.Execute(context.Background(), Command{
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

func TestExecute_RepositoryError(t *testing.T) {
	h := New(Options{
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

	_, _, err := h.Execute(context.Background(), Command{
		WorkspaceID: "ws_1",
		UserID:      "user_1",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestExecute_FilterPassthrough(t *testing.T) {
	var capturedFilter auditdomain.AuditFilter
	from := now()
	to := now().Add(time.Hour)
	h := New(Options{
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

	_, _, err := h.Execute(context.Background(), Command{
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

func TestExecute_ValidLimit100(t *testing.T) {
	h := New(Options{
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

	_, _, err := h.Execute(context.Background(), Command{
		WorkspaceID: "ws_1",
		UserID:      "user_1",
		Limit:       100,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExecute_PermissionCheckCalled(t *testing.T) {
	permChecked := false
	h := New(Options{
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

	_, _, err := h.Execute(context.Background(), Command{
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
