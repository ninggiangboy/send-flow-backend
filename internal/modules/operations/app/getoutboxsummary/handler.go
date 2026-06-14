package getoutboxsummary

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
	Filter      domain.OutboxFilter
}

func New(opts Options) *Handler {
	return &Handler{
		outboxRepo:    opts.OutboxRepo,
		accessChecker: opts.AccessChecker,
		log:           opts.Logger.With("usecase", "get_outbox_summary"),
	}
}

func (h *Handler) Execute(ctx context.Context, input Input) (domain.OutboxSummary, error) {
	log := h.log.With("workspace_id", input.WorkspaceID)
	if err := h.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, "operations.queue.read"); err != nil {
		if errors.Is(err, domain.ErrQueueReadDenied) || errors.Is(err, domain.ErrDLQReadDenied) || errors.Is(err, domain.ErrReplayManageDenied) {
			return domain.OutboxSummary{}, err
		}
		return domain.OutboxSummary{}, err
	}
	if input.WorkspaceID == "" {
		log.Warn("missing workspace id")
		return domain.OutboxSummary{}, domain.ErrWorkspaceRequired
	}
	if err := input.Filter.Validate(); err != nil {
		log.Warn("invalid outbox filter", "error", err)
		return domain.OutboxSummary{}, err
	}
	return h.outboxRepo.GetSummary(ctx, input.WorkspaceID, input.Filter)
}
