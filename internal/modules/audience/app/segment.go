package app

import (
	"context"
	"errors"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/ports"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/constants"
)

func (s *Service) CreateSegment(ctx context.Context, workspaceID, userID, name string, definition map[string]any, now time.Time) (*SegmentResult, error) {
	log := s.log.With("usecase", "create_segment", "workspace_id", workspaceID)
	if err := s.accessChecker.RequirePermission(ctx, workspaceID, userID, "audience.write"); err != nil {
		if errors.Is(err, domain.ErrWriteDenied) {
			return nil, err
		}
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, domain.ErrWriteDenied
		}
		return nil, err
	}

	if name == "" {
		return nil, domain.ErrSegmentDefinitionInvalid
	}
	if definition == nil || len(definition) == 0 {
		return nil, domain.ErrSegmentDefinitionInvalid
	}

	if rules, ok := definition["rules"]; ok {
		if _, ok := rules.([]any); !ok {
			return nil, domain.ErrSegmentDefinitionInvalid
		}
	}

	id, err := s.idGen()
	if err != nil {
		log.Error("failed to generate id", "error", err)
		return nil, err
	}

	segment := domain.Segment{
		ID:             id,
		WorkspaceID:    workspaceID,
		Name:           name,
		DefinitionJSON: definition,
		Status:         domain.SegmentStatusReady,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := s.segmentsWrite.CreateSegment(ctx, segment); err != nil {
		if errors.Is(err, domain.ErrSegmentNameConflict) {
			return nil, err
		}
		log.Error("failed to create segment", "error", err)
		return nil, err
	}

	log.Info("segment created", "segment_id", id)
	return &SegmentResult{Segment: segmentToDTO(segment)}, nil
}

func (s *Service) ListSegments(ctx context.Context, workspaceID, userID, status string, limit int, cursor string) (*SegmentListResult, error) {
	log := s.log.With("usecase", "list_segments", "workspace_id", workspaceID)
	if err := s.accessChecker.RequirePermission(ctx, workspaceID, userID, "audience.read"); err != nil {
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, err
		}
		return nil, err
	}

	if limit <= 0 || limit > 100 {
		limit = constants.DefaultPageSize
	}

	query := ports.SegmentListQuery{WorkspaceID: workspaceID, Status: status, Limit: limit, Cursor: cursor}
	segments, nextCursor, err := s.segmentsRead.ListSegments(ctx, query)
	if err != nil {
		log.Error("failed to list segments", "error", err)
		return nil, err
	}

	return &SegmentListResult{Segments: segmentSliceToDTOs(segments), NextCursor: nextCursor}, nil
}

func (s *Service) UpdateSegment(ctx context.Context, workspaceID, segmentID, userID, name string, definition map[string]any, status string, now time.Time) (*SegmentResult, error) {
	log := s.log.With("usecase", "update_segment", "workspace_id", workspaceID, "segment_id", segmentID)
	if err := s.accessChecker.RequirePermission(ctx, workspaceID, userID, "audience.write"); err != nil {
		if errors.Is(err, domain.ErrWriteDenied) {
			return nil, err
		}
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, domain.ErrWriteDenied
		}
		return nil, err
	}

	segment, err := s.segmentsRead.FindSegmentByID(ctx, workspaceID, segmentID)
	if err != nil {
		if errors.Is(err, domain.ErrSegmentNotFound) {
			return nil, err
		}
		log.Error("failed to find segment", "error", err)
		return nil, err
	}

	if name != "" {
		segment.Name = name
	}
	if definition != nil {
		if len(definition) == 0 {
			return nil, domain.ErrSegmentDefinitionInvalid
		}
		if rules, ok := definition["rules"]; ok {
			if _, ok := rules.([]any); !ok {
				return nil, domain.ErrSegmentDefinitionInvalid
			}
		}
		segment.DefinitionJSON = definition
	}
	if status != "" {
		if !domain.ValidSegmentStatus(status) {
			return nil, domain.ErrContactPayloadInvalid
		}
		segment.Status = domain.SegmentStatus(status)
	}
	segment.UpdatedAt = now

	if err := s.segmentsWrite.UpdateSegment(ctx, *segment); err != nil {
		if errors.Is(err, domain.ErrSegmentNameConflict) {
			return nil, err
		}
		log.Error("failed to update segment", "error", err)
		return nil, err
	}

	log.Info("segment updated")
	return &SegmentResult{Segment: segmentToDTO(*segment)}, nil
}
