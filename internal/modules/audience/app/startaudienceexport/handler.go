package startaudienceexport

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/ports"
)

type Options struct {
	ExportJobsWrite ports.ExportJobWriteRepository
	AccessChecker   ports.WorkspaceAccessChecker
	IDGen           func() (string, error)
	Logger          *slog.Logger
}

type Command struct {
	WorkspaceID    string
	UserID         string
	Format         string
	Filters        map[string]any
	SelectedFields []string
	Now            time.Time
}

type Handler struct {
	exportJobsWrite ports.ExportJobWriteRepository
	accessChecker   ports.WorkspaceAccessChecker
	idGen           func() (string, error)
	log             *slog.Logger
}

func New(opts Options) *Handler {
	return &Handler{
		exportJobsWrite: opts.ExportJobsWrite,
		accessChecker:   opts.AccessChecker,
		idGen:           opts.IDGen,
		log:             opts.Logger.With("usecase", "start_audience_export"),
	}
}

func (h *Handler) Execute(ctx context.Context, cmd Command) (*domain.AudienceExportJob, error) {
	if err := h.accessChecker.RequirePermission(ctx, cmd.WorkspaceID, cmd.UserID, "audience.export"); err != nil {
		if errors.Is(err, domain.ErrExportDenied) {
			return nil, err
		}
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, domain.ErrExportDenied
		}
		return nil, err
	}

	if !domain.ValidExportFormat(cmd.Format) {
		return nil, domain.ErrExportFormatInvalid
	}

	id, err := h.idGen()
	if err != nil {
		h.log.Error("failed to generate id", "error", err)
		return nil, err
	}

	job := domain.AudienceExportJob{
		ID:             id,
		WorkspaceID:    cmd.WorkspaceID,
		FiltersJSON:    cmd.Filters,
		SelectedFields: cmd.SelectedFields,
		Format:         domain.ExportFormat(cmd.Format),
		Status:         domain.JobStatusQueued,
		CreatedAt:      cmd.Now,
		UpdatedAt:      cmd.Now,
	}

	if err := h.exportJobsWrite.CreateExportJob(ctx, job); err != nil {
		h.log.Error("failed to create export job", "error", err)
		return nil, err
	}

	h.log.Info("export job created", "export_id", id)
	return &job, nil
}
