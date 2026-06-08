package app

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/suppression/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/suppression/ports"
)

type mockSuppressionReadRepo struct {
	ports.SuppressionReadRepository
	findByID          func(ctx context.Context, workspaceID, entryID string) (*domain.SuppressionEntry, error)
	list              func(ctx context.Context, query ports.SuppressionListQuery) ([]domain.SuppressionEntry, string, error)
	findActiveByEmail func(ctx context.Context, query ports.SuppressionCheckQuery) (*domain.SuppressionEntry, error)
}

func (m *mockSuppressionReadRepo) FindByID(ctx context.Context, workspaceID, entryID string) (*domain.SuppressionEntry, error) {
	return m.findByID(ctx, workspaceID, entryID)
}

func (m *mockSuppressionReadRepo) List(ctx context.Context, query ports.SuppressionListQuery) ([]domain.SuppressionEntry, string, error) {
	return m.list(ctx, query)
}

func (m *mockSuppressionReadRepo) FindActiveByEmail(ctx context.Context, query ports.SuppressionCheckQuery) (*domain.SuppressionEntry, error) {
	return m.findActiveByEmail(ctx, query)
}

type mockSuppressionWriteRepo struct {
	ports.SuppressionWriteRepository
	create func(ctx context.Context, entry domain.SuppressionEntry) error
	remove func(ctx context.Context, workspaceID, entryID string, removedAt time.Time) error
}

func (m *mockSuppressionWriteRepo) Create(ctx context.Context, entry domain.SuppressionEntry) error {
	return m.create(ctx, entry)
}

func (m *mockSuppressionWriteRepo) Remove(ctx context.Context, workspaceID, entryID string, removedAt time.Time) error {
	return m.remove(ctx, workspaceID, entryID, removedAt)
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestCreateSystemEntry_CreatesWithoutPermission(t *testing.T) {
	now := time.Now().UTC()
	createdEntry := &domain.SuppressionEntry{}

	svc := NewService(Options{
		EntriesRead: &mockSuppressionReadRepo{
			findActiveByEmail: func(ctx context.Context, query ports.SuppressionCheckQuery) (*domain.SuppressionEntry, error) {
				return nil, nil
			},
		},
		EntriesWrite: &mockSuppressionWriteRepo{
			create: func(ctx context.Context, entry domain.SuppressionEntry) error {
				createdEntry = &entry
				return nil
			},
		},
		IDGen:  func() (string, error) { return "sup_1", nil },
		Logger: testLogger(),
	})

	entry, created, err := svc.CreateSystemEntry(context.Background(), CreateSystemEntryInput{
		WorkspaceID:     "ws_1",
		Email:           "test@example.com",
		EmailNormalized: "test@example.com",
		Scope:           "workspace",
		Reason:          "bounce",
		Source:          "provider_event",
		SourceEventID:   "norm_1",
		Note:            "auto-suppressed",
		Now:             now,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry == nil {
		t.Fatal("expected entry, got nil")
	}
	if entry.ID != "sup_1" {
		t.Errorf("expected ID sup_1, got %s", entry.ID)
	}
	if !created {
		t.Fatal("expected created to be true")
	}
	if createdEntry.ID != "sup_1" {
		t.Errorf("expected created ID sup_1, got %s", createdEntry.ID)
	}
}

func TestCreateSystemEntry_DuplicateIsIdempotent(t *testing.T) {
	now := time.Now().UTC()
	existingEntry := &domain.SuppressionEntry{
		ID:              "sup_existing",
		WorkspaceID:     "ws_1",
		EmailNormalized: "test@example.com",
		Scope:           domain.SuppressionScopeWorkspace,
		Reason:          domain.SuppressionReasonBounce,
		Status:          domain.SuppressionStatusActive,
	}

	createCalled := false

	svc := NewService(Options{
		EntriesRead: &mockSuppressionReadRepo{
			findActiveByEmail: func(ctx context.Context, query ports.SuppressionCheckQuery) (*domain.SuppressionEntry, error) {
				return existingEntry, nil
			},
		},
		EntriesWrite: &mockSuppressionWriteRepo{
			create: func(ctx context.Context, entry domain.SuppressionEntry) error {
				createCalled = true
				return nil
			},
		},
		IDGen:  func() (string, error) { return "sup_new", nil },
		Logger: testLogger(),
	})

	entry, created, err := svc.CreateSystemEntry(context.Background(), CreateSystemEntryInput{
		WorkspaceID:     "ws_1",
		Email:           "test@example.com",
		EmailNormalized: "test@example.com",
		Scope:           "workspace",
		Reason:          "bounce",
		Now:             now,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry.ID != "sup_existing" {
		t.Errorf("expected existing entry ID sup_existing, got %s", entry.ID)
	}
	if created {
		t.Fatal("expected created to be false for duplicate")
	}
	if createCalled {
		t.Fatal("expected no create call for duplicate")
	}
}

func TestCreateSystemEntry_InvalidScope(t *testing.T) {
	svc := NewService(Options{
		Logger: testLogger(),
	})

	_, _, err := svc.CreateSystemEntry(context.Background(), CreateSystemEntryInput{
		WorkspaceID:     "ws_1",
		EmailNormalized: "test@example.com",
		Scope:           "invalid_scope",
		Reason:          "bounce",
	})
	if !errors.Is(err, domain.ErrScopeInvalid) {
		t.Errorf("expected ErrScopeInvalid, got %v", err)
	}
}

func TestCreateSystemEntry_InvalidReason(t *testing.T) {
	svc := NewService(Options{
		Logger: testLogger(),
	})

	_, _, err := svc.CreateSystemEntry(context.Background(), CreateSystemEntryInput{
		WorkspaceID:     "ws_1",
		EmailNormalized: "test@example.com",
		Scope:           "workspace",
		Reason:          "invalid_reason",
	})
	if !errors.Is(err, domain.ErrReasonInvalid) {
		t.Errorf("expected ErrReasonInvalid, got %v", err)
	}
}

func TestCreateSystemEntry_EmptyEmailNormalized(t *testing.T) {
	svc := NewService(Options{
		Logger: testLogger(),
	})

	_, _, err := svc.CreateSystemEntry(context.Background(), CreateSystemEntryInput{
		WorkspaceID:     "ws_1",
		EmailNormalized: "",
		Scope:           "workspace",
		Reason:          "bounce",
	})
	if !errors.Is(err, domain.ErrScopeInvalid) {
		t.Errorf("expected ErrScopeInvalid for empty email, got %v", err)
	}
}
