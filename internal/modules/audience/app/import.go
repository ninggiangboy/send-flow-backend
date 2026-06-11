package app

import (
	"context"
	"errors"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/ports"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/constants"
)

func (s *Service) StartAudienceImport(ctx context.Context, workspaceID, userID, sourceURI, dedupeMode string, metadata map[string]any, now time.Time) (*ImportJobResult, error) {
	log := s.log.With("usecase", "start_audience_import", "workspace_id", workspaceID)
	if err := s.accessChecker.RequirePermission(ctx, workspaceID, userID, "audience.import"); err != nil {
		if errors.Is(err, domain.ErrImportDenied) {
			return nil, err
		}
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, domain.ErrImportDenied
		}
		return nil, err
	}

	if sourceURI == "" {
		return nil, domain.ErrImportSourceInvalid
	}

	if !domain.ValidDedupeMode(dedupeMode) {
		return nil, domain.ErrImportSourceInvalid
	}

	id, err := s.idGen()
	if err != nil {
		log.Error("failed to generate id", "error", err)
		return nil, err
	}

	job := domain.AudienceImportJob{
		ID:          id,
		WorkspaceID: workspaceID,
		SourceURI:   sourceURI,
		DedupeMode:  domain.DedupeMode(dedupeMode),
		Status:      domain.JobStatusQueued,
		Metadata:    metadata,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := s.importJobsWrite.CreateImportJob(ctx, job); err != nil {
		log.Error("failed to create import job", "error", err)
		return nil, err
	}

	log.Info("import job created", "import_id", id)
	return &ImportJobResult{Job: importJobToDTO(job)}, nil
}

func (s *Service) ListAudienceImports(ctx context.Context, workspaceID, userID, status string, limit int, cursor string) (*ImportJobListResult, error) {
	log := s.log.With("usecase", "list_audience_imports", "workspace_id", workspaceID)
	if err := s.accessChecker.RequirePermission(ctx, workspaceID, userID, "audience.read"); err != nil {
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, err
		}
		return nil, err
	}

	if limit <= 0 || limit > 100 {
		limit = constants.DefaultPageSize
	}

	query := ports.ImportJobListQuery{WorkspaceID: workspaceID, Status: status, Limit: limit, Cursor: cursor}
	jobs, nextCursor, err := s.importJobsRead.ListImportJobs(ctx, query)
	if err != nil {
		log.Error("failed to list import jobs", "error", err)
		return nil, err
	}

	return &ImportJobListResult{Jobs: importJobSliceToDTOs(jobs), NextCursor: nextCursor}, nil
}

func (s *Service) GetAudienceImport(ctx context.Context, workspaceID, jobID, userID string) (*ImportJobResult, error) {
	log := s.log.With("usecase", "get_audience_import", "workspace_id", workspaceID, "import_id", jobID)
	if err := s.accessChecker.RequirePermission(ctx, workspaceID, userID, "audience.read"); err != nil {
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, err
		}
		return nil, err
	}

	job, err := s.importJobsRead.FindImportJobByID(ctx, workspaceID, jobID)
	if err != nil {
		if errors.Is(err, domain.ErrImportJobNotFound) {
			return nil, err
		}
		log.Error("failed to find import job", "error", err)
		return nil, err
	}

	return &ImportJobResult{Job: importJobToDTO(*job)}, nil
}
