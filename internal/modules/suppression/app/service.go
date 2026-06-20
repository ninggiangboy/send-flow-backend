package app

import (
	"context"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/suppression/app/entry"
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
	listEntriesH       *entry.ListHandler
	createEntryH       *entry.CreateHandler
	createSystemEntryH *entry.CreateSystemHandler
	checkSuppressionH  *entry.CheckHandler
	removeEntryH       *entry.RemoveHandler
}

func NewService(opts Options) *Service {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.IDGen == nil {
		opts.IDGen = func() (string, error) { return "", nil }
	}
	return &Service{
		listEntriesH: entry.NewListHandler(struct {
			EntriesRead   ports.SuppressionReadRepository
			AccessChecker ports.WorkspaceAccessChecker
			Logger        *slog.Logger
		}{
			EntriesRead:   opts.EntriesRead,
			AccessChecker: opts.AccessChecker,
			Logger:        opts.Logger,
		}),
		createEntryH: entry.NewCreateHandler(struct {
			EntriesWrite  ports.SuppressionWriteRepository
			AccessChecker ports.WorkspaceAccessChecker
			IDGen         func() (string, error)
			Logger        *slog.Logger
		}{
			EntriesWrite:  opts.EntriesWrite,
			AccessChecker: opts.AccessChecker,
			IDGen:         opts.IDGen,
			Logger:        opts.Logger,
		}),
		createSystemEntryH: entry.NewCreateSystemHandler(struct {
			EntriesWrite ports.SuppressionWriteRepository
			IDGen        func() (string, error)
			Logger       *slog.Logger
		}{
			EntriesWrite: opts.EntriesWrite,
			IDGen:        opts.IDGen,
			Logger:       opts.Logger,
		}),
		checkSuppressionH: entry.NewCheckHandler(struct {
			EntriesRead ports.SuppressionReadRepository
			Logger      *slog.Logger
		}{
			EntriesRead: opts.EntriesRead,
			Logger:      opts.Logger,
		}),
		removeEntryH: entry.NewRemoveHandler(struct {
			EntriesWrite  ports.SuppressionWriteRepository
			AccessChecker ports.WorkspaceAccessChecker
			Logger        *slog.Logger
		}{
			EntriesWrite:  opts.EntriesWrite,
			AccessChecker: opts.AccessChecker,
			Logger:        opts.Logger,
		}),
	}
}

func (s *Service) ListEntries(ctx context.Context, query ports.SuppressionListQuery, userID string) (*ListEntriesResult, error) {
	entries, cursor, err := s.listEntriesH.Execute(ctx, entry.ListQuery{
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
	return s.createEntryH.Execute(ctx, input)
}

func (s *Service) CreateSystemEntry(ctx context.Context, input CreateSystemEntryInput) (*domain.SuppressionEntry, bool, error) {
	return s.createSystemEntryH.Execute(ctx, input)
}

func (s *Service) CheckSuppression(ctx context.Context, workspaceID, emailNormalized, scope string) (*CheckSuppressionResult, error) {
	return s.checkSuppressionH.Execute(ctx, entry.CheckQuery{
		WorkspaceID:     workspaceID,
		EmailNormalized: emailNormalized,
		Scope:           scope,
	})
}

func (s *Service) RemoveEntry(ctx context.Context, workspaceID, entryID, userID string, now time.Time) error {
	return s.removeEntryH.Execute(ctx, entry.RemoveInput{
		WorkspaceID: workspaceID,
		EntryID:     entryID,
		UserID:      userID,
		Now:         now,
	})
}
