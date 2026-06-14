package listoutboxrecords

import (
	"context"
	"log/slog"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/operations/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/operations/ports"
)

type Options struct {
	OutboxRepo    ports.OutboxRepository
	AccessChecker ports.WorkspaceAccessChecker
	Logger        *slog.Logger
}

type Handler struct {
	outboxRepo    ports.OutboxRepository
	accessChecker ports.WorkspaceAccessChecker
	log           *slog.Logger
}

type Input struct {
	WorkspaceID string
	UserID      string
	Filter      domain.OutboxFilter
}

func New(opts Options) *Handler {
	return &Handler{
		outboxRepo:    opts.OutboxRepo,
		accessChecker: opts.AccessChecker,
		log:           opts.Logger.With("usecase", "list_outbox_records"),
	}
}

func (h *Handler) Execute(ctx context.Context, input Input) ([]domain.OutboxRecord, string, error) {
	log := h.log.With("workspace_id", input.WorkspaceID)
	if err := h.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, "operations.queue.read"); err != nil {
		return nil, "", err
	}
	if input.WorkspaceID == "" {
		return nil, "", domain.ErrWorkspaceRequired
	}
	if err := input.Filter.Validate(); err != nil {
		log.Warn("invalid outbox filter", "error", err)
		return nil, "", err
	}
	records, cursor, err := h.outboxRepo.List(ctx, input.WorkspaceID, input.Filter)
	if err != nil {
		log.Error("failed to list outbox records", "error", err)
		return nil, "", err
	}
	for i := range records {
		records[i].Payload = domain.RedactSensitiveFields(domain.SanitizePayloadPreview(records[i].Payload, 4096))
		if len(records[i].Headers) > 0 && string(records[i].Headers) != "null" {
			records[i].Headers = domain.RedactSensitiveFields(domain.SanitizePayloadPreview(records[i].Headers, 2048))
		}
	}
	return records, cursor, nil
}
