package app

import (
	"context"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/suppression/app/checksuppression"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/suppression/app/createentry"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/suppression/app/createsystementry"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/suppression/app/listentries"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/suppression/app/removeentry"
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
	listEntriesH       *listentries.Handler
	createEntryH       *createentry.Handler
	createSystemEntryH *createsystementry.Handler
	checkSuppressionH  *checksuppression.Handler
	removeEntryH       *removeentry.Handler
}

func NewService(opts Options) *Service {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.IDGen == nil {
		opts.IDGen = func() (string, error) { return "", nil }
	}
	return &Service{
		listEntriesH: listentries.New(listentries.Options{
			EntriesRead:   opts.EntriesRead,
			AccessChecker: opts.AccessChecker,
			Logger:        opts.Logger,
		}),
		createEntryH: createentry.New(createentry.Options{
			EntriesRead:   opts.EntriesRead,
			EntriesWrite:  opts.EntriesWrite,
			AccessChecker: opts.AccessChecker,
			IDGen:         opts.IDGen,
			Logger:        opts.Logger,
		}),
		createSystemEntryH: createsystementry.New(createsystementry.Options{
			EntriesRead:  opts.EntriesRead,
			EntriesWrite: opts.EntriesWrite,
			IDGen:        opts.IDGen,
			Logger:       opts.Logger,
		}),
		checkSuppressionH: checksuppression.New(checksuppression.Options{
			EntriesRead: opts.EntriesRead,
			Logger:      opts.Logger,
		}),
		removeEntryH: removeentry.New(removeentry.Options{
			EntriesRead:   opts.EntriesRead,
			EntriesWrite:  opts.EntriesWrite,
			AccessChecker: opts.AccessChecker,
			Logger:        opts.Logger,
		}),
	}
}

type ListEntriesResult struct {
	Entries    []domain.SuppressionEntry
	NextCursor string
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

type CreateSystemEntryInput struct {
	WorkspaceID     string
	Email           string
	EmailNormalized string
	Scope           string
	Reason          string
	Source          string
	SourceEventID   string
	Note            string
	Now             time.Time
}

type CheckSuppressionResult struct {
	Suppressed bool
	Reason     string
	Scope      string
	EntryID    string
}

func (s *Service) ListEntries(ctx context.Context, query ports.SuppressionListQuery, userID string) (*ListEntriesResult, error) {
	entries, cursor, err := s.listEntriesH.Execute(ctx, listentries.Command{
		WorkspaceID: query.WorkspaceID,
		UserID:      userID,
		Query:       query,
	})
	if err != nil {
		return nil, err
	}
	return &ListEntriesResult{Entries: entries, NextCursor: cursor}, nil
}

func (s *Service) CreateEntry(ctx context.Context, input CreateEntryInput) (*domain.SuppressionEntry, error) {
	return s.createEntryH.Execute(ctx, createentry.Command{
		WorkspaceID: input.WorkspaceID,
		UserID:      input.UserID,
		Email:       input.Email,
		Scope:       input.Scope,
		Reason:      input.Reason,
		Note:        input.Note,
		Now:         input.Now,
	})
}

func (s *Service) CreateSystemEntry(ctx context.Context, input CreateSystemEntryInput) (*domain.SuppressionEntry, bool, error) {
	return s.createSystemEntryH.Execute(ctx, createsystementry.Command{
		WorkspaceID:     input.WorkspaceID,
		Email:           input.Email,
		EmailNormalized: input.EmailNormalized,
		Scope:           input.Scope,
		Reason:          input.Reason,
		Source:          input.Source,
		SourceEventID:   input.SourceEventID,
		Note:            input.Note,
		Now:             input.Now,
	})
}

func (s *Service) CheckSuppression(ctx context.Context, workspaceID, emailNormalized, scope string) (*CheckSuppressionResult, error) {
	result, err := s.checkSuppressionH.Execute(ctx, checksuppression.Command{
		WorkspaceID:     workspaceID,
		EmailNormalized: emailNormalized,
		Scope:           scope,
	})
	if err != nil {
		return nil, err
	}
	return &CheckSuppressionResult{
		Suppressed: result.Suppressed,
		Reason:     result.Reason,
		Scope:      result.Scope,
		EntryID:    result.EntryID,
	}, nil
}

func (s *Service) RemoveEntry(ctx context.Context, workspaceID, entryID, userID string, now time.Time) error {
	return s.removeEntryH.Execute(ctx, removeentry.Command{
		WorkspaceID: workspaceID,
		EntryID:     entryID,
		UserID:      userID,
		Now:         now,
	})
}
