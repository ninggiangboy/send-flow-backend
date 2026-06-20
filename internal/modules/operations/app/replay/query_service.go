package replay

import (
	"context"
	"errors"
	"log/slog"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/operations/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/operations/ports"
)

type QueryOptions struct {
	ReplayJobRepo ports.ReplayJobRepository
	AccessChecker ports.WorkspaceAccessChecker
	Logger        *slog.Logger
}

type GetInput struct {
	WorkspaceID string
	UserID      string
	JobID       string
}

type ListInput struct {
	WorkspaceID string
	UserID      string
	Filter      domain.ReplayJobFilter
}

type QueryService struct {
	replayJobRepo ports.ReplayJobRepository
	accessChecker ports.WorkspaceAccessChecker
	log           *slog.Logger
}

func NewQueryService(opts QueryOptions) *QueryService {
	return &QueryService{
		replayJobRepo: opts.ReplayJobRepo,
		accessChecker: opts.AccessChecker,
		log:           opts.Logger.With("usecase", "replay_query"),
	}
}

func (s *QueryService) GetJob(ctx context.Context, input GetInput) (*domain.ReplayJob, error) {
	log := s.log.With("workspace_id", input.WorkspaceID, "replay_job_id", input.JobID)
	if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, "operations.replay.manage"); err != nil {
		return nil, err
	}
	if input.WorkspaceID == "" {
		return nil, domain.ErrWorkspaceRequired
	}
	job, err := s.replayJobRepo.FindByID(ctx, input.WorkspaceID, input.JobID)
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

func (s *QueryService) ListJobs(ctx context.Context, input ListInput) ([]domain.ReplayJob, string, error) {
	log := s.log.With("workspace_id", input.WorkspaceID)
	if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, "operations.replay.manage"); err != nil {
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
	jobs, cursor, err := s.replayJobRepo.List(ctx, input.WorkspaceID, input.Filter)
	if err != nil {
		log.Error("failed to list replay jobs", "error", err)
		return nil, "", err
	}
	return jobs, cursor, nil
}
