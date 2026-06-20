package app

import (
	"context"
	"log/slog"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/audit/app/entry"
	auditdomain "github.com/ninggiangboy/send-flow/backend/internal/modules/audit/domain"
	auditports "github.com/ninggiangboy/send-flow/backend/internal/modules/audit/ports"
)

type Options struct {
	EntriesRead  auditports.EntryReadRepository
	EntriesWrite auditports.EntryWriteRepository
	PermChecker  auditports.PermissionChecker
	IDGen        func() (string, error)
	Logger       *slog.Logger
}

type Service struct {
	recordEntry *entry.RecordHandler
	listEntries *entry.ListHandler
}

func NewService(opts Options) *Service {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	return &Service{
		recordEntry: entry.NewRecordHandler(struct {
			EntriesWrite auditports.EntryWriteRepository
			IDGen        func() (string, error)
			Logger       *slog.Logger
		}{
			EntriesWrite: opts.EntriesWrite,
			IDGen:        opts.IDGen,
			Logger:       opts.Logger,
		}),
		listEntries: entry.NewListHandler(struct {
			EntriesRead auditports.EntryReadRepository
			PermChecker auditports.PermissionChecker
			Logger      *slog.Logger
		}{
			EntriesRead: opts.EntriesRead,
			PermChecker: opts.PermChecker,
			Logger:      opts.Logger,
		}),
	}
}

func (s *Service) RecordAuditEntry(ctx context.Context, input RecordAuditEntryInput) error {
	return s.recordEntry.Execute(ctx, entry.RecordInput{
		WorkspaceID:    input.WorkspaceID,
		ActorUserID:    input.ActorUserID,
		ActionType:     input.ActionType,
		TargetType:     input.TargetType,
		TargetID:       input.TargetID,
		RequestID:      input.RequestID,
		PayloadSummary: input.PayloadSummary,
		OccurredAt:     input.OccurredAt,
	})
}

func (s *Service) ListAuditEntries(ctx context.Context, input ListAuditEntriesInput) ([]auditdomain.AuditEntry, string, error) {
	return s.listEntries.Execute(ctx, input)
}
