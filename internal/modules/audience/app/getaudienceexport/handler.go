package getaudienceexport

import (
	"context"
	"errors"
	"log/slog"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/ports"
)

type Options struct {
	ExportJobsRead ports.ExportJobReadRepository
	AccessChecker  ports.WorkspaceAccessChecker
	Logger         *slog.Logger
}

type Command struct {
	WorkspaceID string
	JobID       string
	UserID      string
}

type Handler struct {
	exportJobsRead ports.ExportJobReadRepository
	accessChecker  ports.WorkspaceAccessChecker
	log            *slog.Logger
}

func New(opts Options) *Handler {
	return &Handler{
		exportJobsRead: opts.ExportJobsRead,
		accessChecker:  opts.AccessChecker,
		log:            opts.Logger.With("usecase", "get_audience_export"),
	}
}

func (h *Handler) Execute(ctx context.Context, cmd Command) (*domain.AudienceExportJob, error) {
	if err := h.accessChecker.RequirePermission(ctx, cmd.WorkspaceID, cmd.UserID, "audience.read"); err != nil {
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, err
		}
		return nil, err
	}

	job, err := h.exportJobsRead.FindExportJobByID(ctx, cmd.WorkspaceID, cmd.JobID)
	if err != nil {
		if errors.Is(err, domain.ErrExportJobNotFound) {
			return nil, err
		}
		h.log.Error("failed to find export job", "error", err)
		return nil, err
	}

	return job, nil
}
