package app

import (
	"context"
	"errors"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/domain"
)

func (s *Service) StartAudienceExport(ctx context.Context, workspaceID, userID, format string, filters map[string]any, selectedFields []string, now time.Time) (*ExportJobResult, error) {
	log := s.log.With("usecase", "start_audience_export", "workspace_id", workspaceID)
	if err := s.accessChecker.RequirePermission(ctx, workspaceID, userID, "audience.export"); err != nil {
		if errors.Is(err, domain.ErrExportDenied) {
			return nil, err
		}
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, domain.ErrExportDenied
		}
		return nil, err
	}

	if !domain.ValidExportFormat(format) {
		return nil, domain.ErrExportFormatInvalid
	}

	id, err := s.idGen()
	if err != nil {
		log.Error("failed to generate id", "error", err)
		return nil, err
	}

	job := domain.AudienceExportJob{
		ID:             id,
		WorkspaceID:    workspaceID,
		FiltersJSON:    filters,
		SelectedFields: selectedFields,
		Format:         domain.ExportFormat(format),
		Status:         domain.JobStatusQueued,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := s.exportJobsWrite.CreateExportJob(ctx, job); err != nil {
		log.Error("failed to create export job", "error", err)
		return nil, err
	}

	log.Info("export job created", "export_id", id)
	return &ExportJobResult{Job: exportJobToDTO(job)}, nil
}

func (s *Service) GetAudienceExport(ctx context.Context, workspaceID, jobID, userID string) (*ExportJobResult, error) {
	log := s.log.With("usecase", "get_audience_export", "workspace_id", workspaceID, "export_id", jobID)
	if err := s.accessChecker.RequirePermission(ctx, workspaceID, userID, "audience.read"); err != nil {
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, err
		}
		return nil, err
	}

	job, err := s.exportJobsRead.FindExportJobByID(ctx, workspaceID, jobID)
	if err != nil {
		if errors.Is(err, domain.ErrExportJobNotFound) {
			return nil, err
		}
		log.Error("failed to find export job", "error", err)
		return nil, err
	}

	return &ExportJobResult{Job: exportJobToDTO(*job)}, nil
}
