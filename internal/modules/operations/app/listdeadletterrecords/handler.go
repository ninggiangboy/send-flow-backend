package listdeadletterrecords

import (
	"context"
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
	Filter      domain.DeadLetterFilter
}

func New(opts Options) *Handler {
	return &Handler{
		deadLetterRepo: opts.DeadLetterRepo,
		accessChecker:  opts.AccessChecker,
		log:            opts.Logger.With("usecase", "list_dead_letter_records"),
	}
}

func (h *Handler) Execute(ctx context.Context, input Input) ([]domain.DeadLetterRecord, string, error) {
	log := h.log.With("workspace_id", input.WorkspaceID)
	if err := h.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, "operations.dlq.read"); err != nil {
		return nil, "", err
	}
	if input.WorkspaceID == "" {
		return nil, "", domain.ErrWorkspaceRequired
	}
	if err := input.Filter.Validate(); err != nil {
		log.Warn("invalid dead letter filter", "error", err)
		return nil, "", err
	}
	records, cursor, err := h.deadLetterRepo.List(ctx, input.WorkspaceID, input.Filter)
	if err != nil {
		log.Error("failed to list dead letter records", "error", err)
		return nil, "", err
	}
	for i := range records {
		records[i].Payload = domain.RedactSensitiveFields(domain.SanitizePayloadPreview(records[i].Payload, 4096))
	}
	return records, cursor, nil
}
