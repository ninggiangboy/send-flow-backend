package entry

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	auditdomain "github.com/ninggiangboy/send-flow/backend/internal/modules/audit/domain"
	auditports "github.com/ninggiangboy/send-flow/backend/internal/modules/audit/ports"
)

type RecordInput struct {
	WorkspaceID    string
	ActorUserID    string
	ActionType     string
	TargetType     string
	TargetID       string
	RequestID      string
	PayloadSummary map[string]any
	OccurredAt     time.Time
}

type RecordHandler struct {
	entriesWrite auditports.EntryWriteRepository
	idGen        func() (string, error)
	log          *slog.Logger
}

func NewRecordHandler(opts struct {
	EntriesWrite auditports.EntryWriteRepository
	IDGen        func() (string, error)
	Logger       *slog.Logger
}) *RecordHandler {
	return &RecordHandler{
		entriesWrite: opts.EntriesWrite,
		idGen:        opts.IDGen,
		log:          opts.Logger.With("usecase", "record_audit_entry"),
	}
}

func (h *RecordHandler) Execute(ctx context.Context, cmd RecordInput) error {
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
