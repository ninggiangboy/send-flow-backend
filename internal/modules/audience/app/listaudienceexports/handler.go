package listaudienceexports

import (
	"context"
	"errors"
	"log/slog"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/ports"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/constants"
)

type Options struct {
	ExportJobsRead ports.ExportJobReadRepository
	AccessChecker  ports.WorkspaceAccessChecker
	Logger         *slog.Logger
}

type Command struct {
	WorkspaceID string
	UserID      string
	Status      string
	Limit       int
	Cursor      string
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
		log:            opts.Logger.With("usecase", "list_audience_exports"),
	}
}

func (h *Handler) Execute(ctx context.Context, cmd Command) ([]domain.AudienceExportJob, string, error) {
	if err := h.accessChecker.RequirePermission(ctx, cmd.WorkspaceID, cmd.UserID, "audience.read"); err != nil {
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, "", err
		}
		return nil, "", err
	}

	limit := cmd.Limit
	if limit <= 0 || limit > 100 {
		limit = constants.DefaultPageSize
	}

	if cmd.Status != "" && !domain.ValidJobStatus(cmd.Status) {
		return nil, "", domain.ErrExportFilterInvalid
	}

	query := ports.ExportJobListQuery{WorkspaceID: cmd.WorkspaceID, Status: cmd.Status, Limit: limit, Cursor: cmd.Cursor}
	jobs, nextCursor, err := h.exportJobsRead.ListExportJobs(ctx, query)
	if err != nil {
		h.log.Error("failed to list export jobs", "error", err)
		return nil, "", err
	}

	return jobs, nextCursor, nil
}
