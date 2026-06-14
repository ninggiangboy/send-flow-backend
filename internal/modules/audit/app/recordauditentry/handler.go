package recordauditentry

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	auditdomain "github.com/ninggiangboy/send-flow/backend/internal/modules/audit/domain"
	auditports "github.com/ninggiangboy/send-flow/backend/internal/modules/audit/ports"
)

type Options struct {
	EntriesWrite auditports.EntryWriteRepository
	IDGen        func() (string, error)
	Logger       *slog.Logger
}

type Command struct {
	WorkspaceID    string
	ActorUserID    string
	ActionType     string
	TargetType     string
	TargetID       string
	RequestID      string
	PayloadSummary map[string]any
	OccurredAt     time.Time
}

type Handler struct {
	entriesWrite auditports.EntryWriteRepository
	idGen        func() (string, error)
	log          *slog.Logger
}

func New(opts Options) *Handler {
	return &Handler{
		entriesWrite: opts.EntriesWrite,
		idGen:        opts.IDGen,
		log:          opts.Logger.With("usecase", "record_audit_entry"),
	}
}

func (h *Handler) Execute(ctx context.Context, cmd Command) error {
	idVal, err := h.idGen()
	if err != nil {
		return fmt.Errorf("generate audit entry id: %w", err)
	}
	payload := auditdomain.RedactPayload(cmd.PayloadSummary)
	entry, err := auditdomain.NewAuditEntry(idVal, cmd.WorkspaceID, cmd.ActorUserID, cmd.ActionType, cmd.TargetType, cmd.TargetID, cmd.RequestID, payload, cmd.OccurredAt)
	if err != nil {
		return err
	}
	if err := h.entriesWrite.Append(ctx, *entry); err != nil {
		h.log.Error("failed to append audit entry", "error", err, "workspace_id", cmd.WorkspaceID, "action_type", cmd.ActionType)
		return err
	}
	h.log.Info("audit entry recorded", "workspace_id", cmd.WorkspaceID, "action_type", cmd.ActionType, "target_type", cmd.TargetType, "target_id", cmd.TargetID)
	return nil
}
