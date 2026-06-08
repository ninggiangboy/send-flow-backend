package app

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/suppression/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/suppression/ports"
)

type Options struct {
	EntriesRead   ports.SuppressionReadRepository
	EntriesWrite  ports.SuppressionWriteRepository
	AccessChecker ports.WorkspaceAccessChecker
	IDGen         func() (string, error)
	Logger        *slog.Logger
}

type Service struct {
	entriesRead   ports.SuppressionReadRepository
	entriesWrite  ports.SuppressionWriteRepository
	accessChecker ports.WorkspaceAccessChecker
	idGen         func() (string, error)
	log           *slog.Logger
}

func NewService(opts Options) *Service {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.IDGen == nil {
		opts.IDGen = func() (string, error) { return "", nil }
	}
	return &Service{
		entriesRead:   opts.EntriesRead,
		entriesWrite:  opts.EntriesWrite,
		accessChecker: opts.AccessChecker,
		idGen:         opts.IDGen,
		log:           opts.Logger.With("module", "suppression"),
	}
}

type ListEntriesResult struct {
	Entries    []domain.SuppressionEntry
	NextCursor string
}

func (s *Service) ListEntries(ctx context.Context, query ports.SuppressionListQuery, userID string) (*ListEntriesResult, error) {
	log := s.log.With("usecase", "list_suppression_entries", "workspace_id", query.WorkspaceID)
	if err := s.accessChecker.RequirePermission(ctx, query.WorkspaceID, userID, "suppression.read"); err != nil {
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, err
		}
		return nil, err
	}

	if query.Limit <= 0 || query.Limit > 100 {
		query.Limit = 50
	}

	entries, cursor, err := s.entriesRead.List(ctx, query)
	if err != nil {
		log.Error("failed to list suppression entries", "error", err)
		return nil, err
	}

	return &ListEntriesResult{Entries: entries, NextCursor: cursor}, nil
}

type CreateEntryInput struct {
	WorkspaceID string
	UserID      string
	Email       string
	Scope       string
	Reason      string
	Note        string
	Now         time.Time
}

func (s *Service) CreateEntry(ctx context.Context, input CreateEntryInput) (*domain.SuppressionEntry, error) {
	log := s.log.With("usecase", "create_suppression_entry", "workspace_id", input.WorkspaceID)
	if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, "suppression.manage"); err != nil {
		if errors.Is(err, domain.ErrManageDenied) {
			return nil, err
		}
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, domain.ErrManageDenied
		}
		return nil, err
	}

	if !domain.ValidSuppressionScope(input.Scope) {
		return nil, domain.ErrScopeInvalid
	}
	if !domain.ValidSuppressionReason(input.Reason) {
		return nil, domain.ErrReasonInvalid
	}

	if input.Email == "" {
		return nil, domain.ErrScopeInvalid
	}

	normalized := domain.NormalizeEmail(input.Email)

	id, err := s.idGen()
	if err != nil {
		log.Error("failed to generate id", "error", err)
		return nil, err
	}

	entry := domain.SuppressionEntry{
		ID:              id,
		WorkspaceID:     input.WorkspaceID,
		Email:           input.Email,
		EmailNormalized: normalized,
		Scope:           domain.SuppressionScope(input.Scope),
		Reason:          domain.SuppressionReason(input.Reason),
		Status:          domain.SuppressionStatusActive,
		Note:            input.Note,
		CreatedAt:       input.Now,
		UpdatedAt:       input.Now,
	}

	if err := s.entriesWrite.Create(ctx, entry); err != nil {
		log.Error("failed to create suppression entry", "error", err)
		return nil, err
	}

	log.Info("suppression entry created", "entry_id", id)
	return &entry, nil
}

type CheckSuppressionResult struct {
	Suppressed bool
	Reason     string
	Scope      string
	EntryID    string
}

func (s *Service) CheckSuppression(ctx context.Context, workspaceID, emailNormalized, scope string) (*CheckSuppressionResult, error) {
	log := s.log.With("usecase", "check_suppression", "workspace_id", workspaceID)

	if workspaceID == "" || emailNormalized == "" {
		return &CheckSuppressionResult{}, nil
	}

	if scope == "" {
		scope = "workspace"
	}

	scopes := []string{scope, "global"}
	entry, err := s.entriesRead.FindActiveByEmail(ctx, ports.SuppressionCheckQuery{
		WorkspaceID:     workspaceID,
		EmailNormalized: emailNormalized,
		Scopes:          scopes,
	})
	if err != nil {
		log.Error("failed to check suppression", "error", err)
		return nil, err
	}

	if entry == nil {
		return &CheckSuppressionResult{Suppressed: false}, nil
	}

	return &CheckSuppressionResult{
		Suppressed: true,
		Reason:     string(entry.Reason),
		Scope:      string(entry.Scope),
		EntryID:    entry.ID,
	}, nil
}

func (s *Service) RemoveEntry(ctx context.Context, workspaceID, entryID, userID string, now time.Time) error {
	log := s.log.With("usecase", "remove_suppression_entry", "workspace_id", workspaceID, "entry_id", entryID)
	if err := s.accessChecker.RequirePermission(ctx, workspaceID, userID, "suppression.manage"); err != nil {
		if errors.Is(err, domain.ErrManageDenied) {
			return err
		}
		if errors.Is(err, domain.ErrReadDenied) {
			return domain.ErrManageDenied
		}
		return err
	}

	entry, err := s.entriesRead.FindByID(ctx, workspaceID, entryID)
	if err != nil {
		if errors.Is(err, domain.ErrEntryNotFound) {
			return err
		}
		log.Error("failed to find entry for removal", "error", err)
		return err
	}

	if entry.Status == domain.SuppressionStatusRemoved {
		return domain.ErrUnsuppressConflict
	}

	if err := s.entriesWrite.Remove(ctx, workspaceID, entryID, now); err != nil {
		log.Error("failed to remove suppression entry", "error", err)
		return err
	}

	log.Info("suppression entry removed")
	return nil
}
