package app

import (
	"context"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/audit/app/listauditentries"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/audit/app/recordauditentry"
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

type ListAuditEntriesInput struct {
	WorkspaceID string
	UserID      string
	ActorUserID string
	ActionType  string
	TargetType  string
	TargetID    string
	From        *time.Time
	To          *time.Time
	Limit       int
	Cursor      string
}

type Service struct {
	recordEntry *recordauditentry.Handler
	listEntries *listauditentries.Handler
}

func NewService(opts Options) *Service {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	return &Service{
		recordEntry: recordauditentry.New(recordauditentry.Options{
			EntriesWrite: opts.EntriesWrite,
			IDGen:        opts.IDGen,
			Logger:       opts.Logger,
		}),
		listEntries: listauditentries.New(listauditentries.Options{
			EntriesRead: opts.EntriesRead,
			PermChecker: opts.PermChecker,
			Logger:      opts.Logger,
		}),
	}
}

func (s *Service) RecordAuditEntry(ctx context.Context, input RecordAuditEntryInput) error {
	return s.recordEntry.Execute(ctx, recordauditentry.Command{
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
	return s.listEntries.Execute(ctx, listauditentries.Command{
		WorkspaceID: input.WorkspaceID,
		UserID:      input.UserID,
		ActorUserID: input.ActorUserID,
		ActionType:  input.ActionType,
		TargetType:  input.TargetType,
		TargetID:    input.TargetID,
		From:        input.From,
		To:          input.To,
		Limit:       input.Limit,
		Cursor:      input.Cursor,
	})
}
