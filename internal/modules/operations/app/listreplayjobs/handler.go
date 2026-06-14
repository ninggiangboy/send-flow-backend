package listreplayjobs

import (
	"context"
	"log/slog"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/operations/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/operations/ports"
)

type Options struct {
	ReplayJobRepo ports.ReplayJobRepository
	AccessChecker ports.WorkspaceAccessChecker
	Logger        *slog.Logger
}

type Handler struct {
	replayJobRepo ports.ReplayJobRepository
	accessChecker ports.WorkspaceAccessChecker
	log           *slog.Logger
}

type Input struct {
	WorkspaceID string
	UserID      string
	Filter      domain.ReplayJobFilter
}

func New(opts Options) *Handler {
	return &Handler{
		replayJobRepo: opts.ReplayJobRepo,
		accessChecker: opts.AccessChecker,
		log:           opts.Logger.With("usecase", "list_replay_jobs"),
	}
}

func (h *Handler) Execute(ctx context.Context, input Input) ([]domain.ReplayJob, string, error) {
	log := h.log.With("workspace_id", input.WorkspaceID)
	if err := h.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, "operations.replay.manage"); err != nil {
		return nil, "", err
	}
	if input.WorkspaceID == "" {
		return nil, "", domain.ErrWorkspaceRequired
	}
	if input.Filter.Status != "" && !domain.IsValidReplayStatus(domain.ReplayJobStatus(input.Filter.Status)) {
		return nil, "", domain.ErrFilterInvalid
	}
	if err := input.Filter.Validate(); err != nil {
		log.Warn("invalid replay job filter", "error", err)
		return nil, "", err
	}
	jobs, cursor, err := h.replayJobRepo.List(ctx, input.WorkspaceID, input.Filter)
	if err != nil {
		log.Error("failed to list replay jobs", "error", err)
		return nil, "", err
	}
	return jobs, cursor, nil
}
