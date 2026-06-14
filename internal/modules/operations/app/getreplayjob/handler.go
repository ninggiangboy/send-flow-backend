package getreplayjob

import (
	"context"
	"errors"
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
	JobID       string
}

func New(opts Options) *Handler {
	return &Handler{
		replayJobRepo: opts.ReplayJobRepo,
		accessChecker: opts.AccessChecker,
		log:           opts.Logger.With("usecase", "get_replay_job"),
	}
}

func (h *Handler) Execute(ctx context.Context, input Input) (*domain.ReplayJob, error) {
	log := h.log.With("workspace_id", input.WorkspaceID, "replay_job_id", input.JobID)
	if err := h.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, "operations.replay.manage"); err != nil {
		return nil, err
	}
	if input.WorkspaceID == "" {
		return nil, domain.ErrWorkspaceRequired
	}
	job, err := h.replayJobRepo.FindByID(ctx, input.WorkspaceID, input.JobID)
	if err != nil {
		if errors.Is(err, domain.ErrReplayJobNotFound) {
			log.Warn("replay job not found")
			return nil, err
		}
		log.Error("failed to get replay job", "error", err)
		return nil, err
	}
	return job, nil
}
