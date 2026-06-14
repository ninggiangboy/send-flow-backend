package updatesegment

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/ports"
)

type Options struct {
	SegmentsRead  ports.SegmentReadRepository
	SegmentsWrite ports.SegmentWriteRepository
	AccessChecker ports.WorkspaceAccessChecker
	Logger        *slog.Logger
}

type Command struct {
	WorkspaceID string
	SegmentID   string
	UserID      string
	Name        string
	Definition  map[string]any
	Status      string
	Now         time.Time
}

type Handler struct {
	segmentsRead  ports.SegmentReadRepository
	segmentsWrite ports.SegmentWriteRepository
	accessChecker ports.WorkspaceAccessChecker
	log           *slog.Logger
}

func New(opts Options) *Handler {
	return &Handler{
		segmentsRead:  opts.SegmentsRead,
		segmentsWrite: opts.SegmentsWrite,
		accessChecker: opts.AccessChecker,
		log:           opts.Logger.With("usecase", "update_segment"),
	}
}

func (h *Handler) Execute(ctx context.Context, cmd Command) (*domain.Segment, error) {
	if err := h.accessChecker.RequirePermission(ctx, cmd.WorkspaceID, cmd.UserID, "audience.write"); err != nil {
		if errors.Is(err, domain.ErrWriteDenied) {
			return nil, err
		}
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, domain.ErrWriteDenied
		}
		return nil, err
	}

	segment, err := h.segmentsRead.FindSegmentByID(ctx, cmd.WorkspaceID, cmd.SegmentID)
	if err != nil {
		if errors.Is(err, domain.ErrSegmentNotFound) {
			return nil, err
		}
		h.log.Error("failed to find segment", "error", err)
		return nil, err
	}

	if cmd.Name != "" {
		segment.Name = cmd.Name
	}
	if cmd.Definition != nil {
		if len(cmd.Definition) == 0 {
			return nil, domain.ErrSegmentDefinitionInvalid
		}
		if rules, ok := cmd.Definition["rules"]; ok {
			if _, ok := rules.([]any); !ok {
				return nil, domain.ErrSegmentDefinitionInvalid
			}
		}
		segment.DefinitionJSON = cmd.Definition
	}
	if cmd.Status != "" {
		if !domain.ValidSegmentStatus(cmd.Status) {
			return nil, domain.ErrContactPayloadInvalid
		}
		segment.Status = domain.SegmentStatus(cmd.Status)
	}
	segment.UpdatedAt = cmd.Now

	if err := h.segmentsWrite.UpdateSegment(ctx, *segment); err != nil {
		if errors.Is(err, domain.ErrSegmentNameConflict) {
			return nil, err
		}
		h.log.Error("failed to update segment", "error", err)
		return nil, err
	}

	h.log.Info("segment updated")
	return segment, nil
}
