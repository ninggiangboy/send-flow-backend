package startaudienceimport

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/ports"
)

type Options struct {
	ImportJobsWrite ports.ImportJobWriteRepository
	AccessChecker   ports.WorkspaceAccessChecker
	IDGen           func() (string, error)
	Logger          *slog.Logger
}

type Command struct {
	WorkspaceID string
	UserID      string
	SourceURI   string
	DedupeMode  string
	Metadata    map[string]any
	Now         time.Time
}

type Handler struct {
	importJobsWrite ports.ImportJobWriteRepository
	accessChecker   ports.WorkspaceAccessChecker
	idGen           func() (string, error)
	log             *slog.Logger
}

func New(opts Options) *Handler {
	return &Handler{
		importJobsWrite: opts.ImportJobsWrite,
		accessChecker:   opts.AccessChecker,
		idGen:           opts.IDGen,
		log:             opts.Logger.With("usecase", "start_audience_import"),
	}
}

func (h *Handler) Execute(ctx context.Context, cmd Command) (*domain.AudienceImportJob, error) {
	if err := h.accessChecker.RequirePermission(ctx, cmd.WorkspaceID, cmd.UserID, "audience.import"); err != nil {
		if errors.Is(err, domain.ErrImportDenied) {
			return nil, err
		}
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, domain.ErrImportDenied
		}
		return nil, err
	}

	if cmd.SourceURI == "" {
		return nil, domain.ErrImportSourceInvalid
	}

	if !domain.ValidDedupeMode(cmd.DedupeMode) {
		return nil, domain.ErrImportSourceInvalid
	}

	id, err := h.idGen()
	if err != nil {
		h.log.Error("failed to generate id", "error", err)
		return nil, err
	}

	job := domain.AudienceImportJob{
		ID:          id,
		WorkspaceID: cmd.WorkspaceID,
		SourceURI:   cmd.SourceURI,
		DedupeMode:  domain.DedupeMode(cmd.DedupeMode),
		Status:      domain.JobStatusQueued,
		Metadata:    cmd.Metadata,
		CreatedAt:   cmd.Now,
		UpdatedAt:   cmd.Now,
	}

	if err := h.importJobsWrite.CreateImportJob(ctx, job); err != nil {
		h.log.Error("failed to create import job", "error", err)
		return nil, err
	}

	h.log.Info("import job created", "import_id", id)
	return &job, nil
}
