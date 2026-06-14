package getoutboxrecord

import (
	"context"
	"errors"
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
	OutboxID    string
}

func New(opts Options) *Handler {
	return &Handler{
		outboxRepo:    opts.OutboxRepo,
		accessChecker: opts.AccessChecker,
		log:           opts.Logger.With("usecase", "get_outbox_record"),
	}
}

func (h *Handler) Execute(ctx context.Context, input Input) (*domain.OutboxRecord, error) {
	log := h.log.With("workspace_id", input.WorkspaceID, "outbox_id", input.OutboxID)
	if err := h.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, "operations.queue.read"); err != nil {
		return nil, err
	}
	if input.WorkspaceID == "" {
		return nil, domain.ErrWorkspaceRequired
	}
	rec, err := h.outboxRepo.FindByID(ctx, input.WorkspaceID, input.OutboxID)
	if err != nil {
		if errors.Is(err, domain.ErrOutboxRecordNotFound) {
			log.Warn("outbox record not found")
			return nil, err
		}
		log.Error("failed to get outbox record", "error", err)
		return nil, err
	}
	rec.Payload = domain.RedactSensitiveFields(domain.SanitizePayloadPreview(rec.Payload, 4096))
	if len(rec.Headers) > 0 && string(rec.Headers) != "null" {
		rec.Headers = domain.RedactSensitiveFields(domain.SanitizePayloadPreview(rec.Headers, 2048))
	}
	return rec, nil
}
