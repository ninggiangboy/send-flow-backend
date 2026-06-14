package getdeadletterrecord

import (
	"context"
	"errors"
	"log/slog"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/operations/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/operations/ports"
)

type Options struct {
	DeadLetterRepo ports.DeadLetterRepository
	AccessChecker  ports.WorkspaceAccessChecker
	Logger         *slog.Logger
}

type Handler struct {
	deadLetterRepo ports.DeadLetterRepository
	accessChecker  ports.WorkspaceAccessChecker
	log            *slog.Logger
}

type Input struct {
	WorkspaceID string
	UserID      string
	RecordID    string
}

func New(opts Options) *Handler {
	return &Handler{
		deadLetterRepo: opts.DeadLetterRepo,
		accessChecker:  opts.AccessChecker,
		log:            opts.Logger.With("usecase", "get_dead_letter_record"),
	}
}

func (h *Handler) Execute(ctx context.Context, input Input) (*domain.DeadLetterRecord, error) {
	log := h.log.With("workspace_id", input.WorkspaceID, "dead_letter_id", input.RecordID)
	if err := h.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, "operations.dlq.read"); err != nil {
		return nil, err
	}
	if input.WorkspaceID == "" {
		return nil, domain.ErrWorkspaceRequired
	}
	rec, err := h.deadLetterRepo.FindByID(ctx, input.WorkspaceID, input.RecordID)
	if err != nil {
		if errors.Is(err, domain.ErrDeadLetterRecordNotFound) {
			log.Warn("dead letter record not found")
			return nil, err
		}
		log.Error("failed to get dead letter record", "error", err)
		return nil, err
	}
	rec.Payload = domain.RedactSensitiveFields(domain.SanitizePayloadPreview(rec.Payload, 4096))
	return rec, nil
}
