package app

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/suppression/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/suppression/ports"
)

type mockReadRepo struct {
	ports.SuppressionReadRepository
	findByID func(ctx context.Context, workspaceID, entryID string) (*domain.SuppressionEntry, error)
	list     func(ctx context.Context, query ports.SuppressionListQuery) ([]domain.SuppressionEntry, string, error)
}

func (m *mockReadRepo) FindByID(ctx context.Context, workspaceID, entryID string) (*domain.SuppressionEntry, error) {
	return m.findByID(ctx, workspaceID, entryID)
}

func (m *mockReadRepo) List(ctx context.Context, query ports.SuppressionListQuery) ([]domain.SuppressionEntry, string, error) {
	return m.list(ctx, query)
}

type mockWriteRepo struct {
	ports.SuppressionWriteRepository
	create func(ctx context.Context, entry domain.SuppressionEntry) error
	remove func(ctx context.Context, workspaceID, entryID string, removedAt time.Time) error
}

func (m *mockWriteRepo) Create(ctx context.Context, entry domain.SuppressionEntry) error {
	return m.create(ctx, entry)
}

func (m *mockWriteRepo) Remove(ctx context.Context, workspaceID, entryID string, removedAt time.Time) error {
	return m.remove(ctx, workspaceID, entryID, removedAt)
}

type mockAccessChecker struct {
	requirePermission func(ctx context.Context, workspaceID, userID, permission string) error
}

func (m *mockAccessChecker) RequirePermission(ctx context.Context, workspaceID, userID, permission string) error {
	return m.requirePermission(ctx, workspaceID, userID, permission)
}

func newTestOpts() Options {
	return Options{
		EntriesRead:  &mockReadRepo{},
		EntriesWrite: &mockWriteRepo{},
		IDGen:        func() (string, error) { return "id_1", nil },
		Logger:       slog.Default(),
	}
}

func TestCreateEntry(t *testing.T) {
	opts := newTestOpts()
	opts.AccessChecker = &mockAccessChecker{
		requirePermission: func(ctx context.Context, workspaceID, userID, permission string) error { return nil },
	}
	created := false
	write := opts.EntriesWrite.(*mockWriteRepo)
	write.create = func(ctx context.Context, entry domain.SuppressionEntry) error {
		created = true
		return nil
	}

	svc := NewService(opts)
	entry, err := svc.CreateEntry(context.Background(), CreateEntryInput{
		WorkspaceID: "ws_1",
		UserID:      "user_1",
		Email:       "test@example.com",
		Scope:       "workspace",
		Reason:      "manual_block",
		Now:         time.Now(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !created {
		t.Error("expected entry to be created")
	}
	if entry.ID != "id_1" {
		t.Errorf("expected ID id_1, got %s", entry.ID)
	}
	if string(entry.Scope) != "workspace" {
		t.Errorf("expected scope workspace, got %s", entry.Scope)
	}
}

func TestCreateEntryInvalidScope(t *testing.T) {
	opts := newTestOpts()
	opts.AccessChecker = &mockAccessChecker{
		requirePermission: func(ctx context.Context, workspaceID, userID, permission string) error { return nil },
	}

	svc := NewService(opts)
	_, err := svc.CreateEntry(context.Background(), CreateEntryInput{
		WorkspaceID: "ws_1",
		UserID:      "user_1",
		Email:       "test@example.com",
		Scope:       "invalid_scope",
		Reason:      "manual_block",
		Now:         time.Now(),
	})
	if !errors.Is(err, domain.ErrScopeInvalid) {
		t.Errorf("expected ErrScopeInvalid, got %v", err)
	}
}

func TestCreateEntryInvalidReason(t *testing.T) {
	opts := newTestOpts()
	opts.AccessChecker = &mockAccessChecker{
		requirePermission: func(ctx context.Context, workspaceID, userID, permission string) error { return nil },
	}

	svc := NewService(opts)
	_, err := svc.CreateEntry(context.Background(), CreateEntryInput{
		WorkspaceID: "ws_1",
		UserID:      "user_1",
		Email:       "test@example.com",
		Scope:       "workspace",
		Reason:      "invalid_reason",
		Now:         time.Now(),
	})
	if !errors.Is(err, domain.ErrReasonInvalid) {
		t.Errorf("expected ErrReasonInvalid, got %v", err)
	}
}

func TestListEntries(t *testing.T) {
	opts := newTestOpts()
	opts.AccessChecker = &mockAccessChecker{
		requirePermission: func(ctx context.Context, workspaceID, userID, permission string) error { return nil },
	}
	read := opts.EntriesRead.(*mockReadRepo)
	read.list = func(ctx context.Context, query ports.SuppressionListQuery) ([]domain.SuppressionEntry, string, error) {
		return []domain.SuppressionEntry{
			{ID: "sup_1", Email: "test@example.com", Scope: domain.SuppressionScopeWorkspace, Reason: domain.SuppressionReasonManualBlock, Status: domain.SuppressionStatusActive},
		}, "", nil
	}

	svc := NewService(opts)
	result, err := svc.ListEntries(context.Background(), ports.SuppressionListQuery{WorkspaceID: "ws_1"}, "user_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(result.Entries))
	}
}

func TestRemoveEntry(t *testing.T) {
	opts := newTestOpts()
	opts.AccessChecker = &mockAccessChecker{
		requirePermission: func(ctx context.Context, workspaceID, userID, permission string) error { return nil },
	}
	removed := false
	read := opts.EntriesRead.(*mockReadRepo)
	read.findByID = func(ctx context.Context, workspaceID, entryID string) (*domain.SuppressionEntry, error) {
		return &domain.SuppressionEntry{ID: entryID, Status: domain.SuppressionStatusActive}, nil
	}
	write := opts.EntriesWrite.(*mockWriteRepo)
	write.remove = func(ctx context.Context, workspaceID, entryID string, removedAt time.Time) error {
		removed = true
		return nil
	}

	svc := NewService(opts)
	err := svc.RemoveEntry(context.Background(), "ws_1", "sup_1", "user_1", time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !removed {
		t.Error("expected entry to be removed")
	}
}

func TestRemoveEntryAlreadyRemoved(t *testing.T) {
	opts := newTestOpts()
	opts.AccessChecker = &mockAccessChecker{
		requirePermission: func(ctx context.Context, workspaceID, userID, permission string) error { return nil },
	}
	read := opts.EntriesRead.(*mockReadRepo)
	read.findByID = func(ctx context.Context, workspaceID, entryID string) (*domain.SuppressionEntry, error) {
		return &domain.SuppressionEntry{ID: entryID, Status: domain.SuppressionStatusRemoved}, nil
	}

	svc := NewService(opts)
	err := svc.RemoveEntry(context.Background(), "ws_1", "sup_1", "user_1", time.Now())
	if !errors.Is(err, domain.ErrUnsuppressConflict) {
		t.Errorf("expected ErrUnsuppressConflict, got %v", err)
	}
}

func TestCreateEntryPermissionDenied(t *testing.T) {
	opts := newTestOpts()
	opts.AccessChecker = &mockAccessChecker{
		requirePermission: func(ctx context.Context, workspaceID, userID, permission string) error {
			return domain.ErrManageDenied
		},
	}

	svc := NewService(opts)
	_, err := svc.CreateEntry(context.Background(), CreateEntryInput{
		WorkspaceID: "ws_1",
		UserID:      "user_1",
		Email:       "test@example.com",
		Scope:       "workspace",
		Reason:      "manual_block",
		Now:         time.Now(),
	})
	if !errors.Is(err, domain.ErrManageDenied) {
		t.Errorf("expected ErrManageDenied, got %v", err)
	}
}
