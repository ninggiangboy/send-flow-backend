package listaudienceimports

import (
	"context"
	"errors"
	"log/slog"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/ports"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/constants"
)

type Options struct {
	ImportJobsRead ports.ImportJobReadRepository
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
	importJobsRead ports.ImportJobReadRepository
	accessChecker  ports.WorkspaceAccessChecker
	log            *slog.Logger
}

func New(opts Options) *Handler {
	return &Handler{
		importJobsRead: opts.ImportJobsRead,
		accessChecker:  opts.AccessChecker,
		log:            opts.Logger.With("usecase", "list_audience_imports"),
	}
}

func (h *Handler) Execute(ctx context.Context, cmd Command) ([]domain.AudienceImportJob, string, error) {
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

	query := ports.ImportJobListQuery{WorkspaceID: cmd.WorkspaceID, Status: cmd.Status, Limit: limit, Cursor: cmd.Cursor}
	jobs, nextCursor, err := h.importJobsRead.ListImportJobs(ctx, query)
	if err != nil {
		h.log.Error("failed to list import jobs", "error", err)
		return nil, "", err
	}

	return jobs, nextCursor, nil
}
