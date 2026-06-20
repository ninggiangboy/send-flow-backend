package entry

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

type ListQuery struct {
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

type ListHandler struct {
	entriesRead auditports.EntryReadRepository
	permChecker auditports.PermissionChecker
	log         *slog.Logger
}

func NewListHandler(opts struct {
	EntriesRead auditports.EntryReadRepository
	PermChecker auditports.PermissionChecker
	Logger      *slog.Logger
}) *ListHandler {
	return &ListHandler{
		entriesRead: opts.EntriesRead,
		permChecker: opts.PermChecker,
		log:         opts.Logger.With("usecase", "list_audit_entries"),
	}
}

func (h *ListHandler) Execute(ctx context.Context, cmd ListQuery) ([]auditdomain.AuditEntry, string, error) {
	limit := cmd.Limit
	if limit <= 0 {
		limit = constants.DefaultPageSize
	}
	if limit > 100 {
		return nil, "", fmt.Errorf("%w: limit must be between 1 and 100", auditdomain.ErrAuditFilterInvalid)
	}
	if cmd.From != nil && cmd.To != nil && cmd.From.After(*cmd.To) {
		return nil, "", fmt.Errorf("%w: 'from' cannot be after 'to'", auditdomain.ErrAuditFilterInvalid)
	}
	if err := h.permChecker.RequirePermission(ctx, cmd.WorkspaceID, cmd.UserID, PermissionAuditRead); err != nil {
		if errors.Is(err, auditdomain.ErrAuditReadDenied) {
			return nil, "", auditdomain.ErrAuditReadDenied
		}
		return nil, "", err
	}
	filter := auditdomain.AuditFilter{
		WorkspaceID: cmd.WorkspaceID,
		ActorUserID: cmd.ActorUserID,
		ActionType:  cmd.ActionType,
		TargetType:  cmd.TargetType,
		TargetID:    cmd.TargetID,
		From:        cmd.From,
		To:          cmd.To,
		Limit:       limit,
		Cursor:      cmd.Cursor,
	}
	entries, next, err := h.entriesRead.List(ctx, filter)
	if err != nil {
		h.log.Error("failed to list audit entries", "error", err, "workspace_id", cmd.WorkspaceID)
		return nil, "", err
	}
	return entries, next, nil
}
