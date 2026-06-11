package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	auditdomain "github.com/ninggiangboy/send-flow/backend/internal/modules/audit/domain"
	auditports "github.com/ninggiangboy/send-flow/backend/internal/modules/audit/ports"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/constants"
)

const PermissionAuditRead = "audit:read"

type Options struct {
	EntriesRead  auditports.EntryReadRepository
	EntriesWrite auditports.EntryWriteRepository
	PermChecker  auditports.PermissionChecker
	IDGen        func() (string, error)
	Logger       *slog.Logger
}

type Service struct {
	entriesRead  auditports.EntryReadRepository
	entriesWrite auditports.EntryWriteRepository
	permChecker  auditports.PermissionChecker
	idGen        func() (string, error)
	logger       *slog.Logger
}

func NewService(opts Options) *Service {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	return &Service{
		entriesRead:  opts.EntriesRead,
		entriesWrite: opts.EntriesWrite,
		permChecker:  opts.PermChecker,
		idGen:        opts.IDGen,
		logger:       opts.Logger.With("usecase", "audit"),
	}
}

func (s *Service) RecordAuditEntry(ctx context.Context, input RecordAuditEntryInput) error {
	idVal, err := s.idGen()
	if err != nil {
		return fmt.Errorf("generate audit entry id: %w", err)
	}
	payload := auditdomain.RedactPayload(input.PayloadSummary)
	entry, err := auditdomain.NewAuditEntry(idVal, input.WorkspaceID, input.ActorUserID, input.ActionType, input.TargetType, input.TargetID, input.RequestID, payload, input.OccurredAt)
	if err != nil {
		return err
	}
	if err := s.entriesWrite.Append(ctx, *entry); err != nil {
		s.logger.Error("failed to append audit entry", "error", err, "workspace_id", input.WorkspaceID, "action_type", input.ActionType)
		return err
	}
	s.logger.Info("audit entry recorded", "workspace_id", input.WorkspaceID, "action_type", input.ActionType, "target_type", input.TargetType, "target_id", input.TargetID)
	return nil
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

func (s *Service) ListAuditEntries(ctx context.Context, input ListAuditEntriesInput) ([]auditdomain.AuditEntry, string, error) {
	limit := input.Limit
	if limit <= 0 {
		limit = constants.DefaultPageSize
	}
	if limit > 100 {
		return nil, "", fmt.Errorf("%w: limit must be between 1 and 100", auditdomain.ErrAuditFilterInvalid)
	}
	if input.From != nil && input.To != nil && input.From.After(*input.To) {
		return nil, "", fmt.Errorf("%w: 'from' cannot be after 'to'", auditdomain.ErrAuditFilterInvalid)
	}
	if err := s.permChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, PermissionAuditRead); err != nil {
		if errors.Is(err, auditdomain.ErrAuditReadDenied) {
			return nil, "", auditdomain.ErrAuditReadDenied
		}
		return nil, "", err
	}
	filter := auditdomain.AuditFilter{
		WorkspaceID: input.WorkspaceID,
		ActorUserID: input.ActorUserID,
		ActionType:  input.ActionType,
		TargetType:  input.TargetType,
		TargetID:    input.TargetID,
		From:        input.From,
		To:          input.To,
		Limit:       limit,
		Cursor:      input.Cursor,
	}
	entries, next, err := s.entriesRead.List(ctx, filter)
	if err != nil {
		s.logger.Error("failed to list audit entries", "error", err, "workspace_id", input.WorkspaceID)
		return nil, "", err
	}
	return entries, next, nil
}
