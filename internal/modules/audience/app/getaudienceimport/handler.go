package getaudienceimport

import (
	"context"
	"errors"
	"log/slog"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/ports"
)

type Options struct {
	ImportJobsRead ports.ImportJobReadRepository
	AccessChecker  ports.WorkspaceAccessChecker
	Logger         *slog.Logger
}

type Command struct {
	WorkspaceID string
	JobID       string
	UserID      string
}

type Handler struct {
	importJobsRead ports.ImportJobReadRepository
	accessChecker  ports.WorkspaceAccessChecker
	log            *slog.Logger
}

func New(opts Options) *Handler {
	return &Handler{
		importJobsRead: opts.ImportJobsRead,
		accessChecker:  opts.AccessChecker,
		log:            opts.Logger.With("usecase", "get_audience_import"),
	}
}

func (h *Handler) Execute(ctx context.Context, cmd Command) (*domain.AudienceImportJob, error) {
	if err := h.accessChecker.RequirePermission(ctx, cmd.WorkspaceID, cmd.UserID, "audience.read"); err != nil {
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, err
		}
		return nil, err
	}

	job, err := h.importJobsRead.FindImportJobByID(ctx, cmd.WorkspaceID, cmd.JobID)
	if err != nil {
		if errors.Is(err, domain.ErrImportJobNotFound) {
			return nil, err
		}
		h.log.Error("failed to find import job", "error", err)
		return nil, err
	}

	return job, nil
}
