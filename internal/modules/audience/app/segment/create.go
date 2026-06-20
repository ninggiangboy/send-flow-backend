package segment

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/ports"
)

type CreateOptions struct {
	SegmentsWrite ports.SegmentWriteRepository
	AccessChecker ports.WorkspaceAccessChecker
	IDGen         func() (string, error)
	Logger        *slog.Logger
}

type CreateCommand struct {
	WorkspaceID string
	UserID      string
	Name        string
	Definition  map[string]any
	Now         time.Time
}

type CreateHandler struct {
	segmentsWrite ports.SegmentWriteRepository
	accessChecker ports.WorkspaceAccessChecker
	idGen         func() (string, error)
	log           *slog.Logger
}

func NewCreate(opts CreateOptions) *CreateHandler {
	return &CreateHandler{
		segmentsWrite: opts.SegmentsWrite,
		accessChecker: opts.AccessChecker,
		idGen:         opts.IDGen,
		log:           opts.Logger.With("usecase", "create_segment"),
	}
}

func (h *CreateHandler) Execute(ctx context.Context, cmd CreateCommand) (*domain.Segment, error) {
	if err := h.accessChecker.RequirePermission(ctx, cmd.WorkspaceID, cmd.UserID, "audience.write"); err != nil {
		if errors.Is(err, domain.ErrWriteDenied) {
			return nil, err
		}
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, domain.ErrWriteDenied
		}
		return nil, err
	}

	if cmd.Name == "" {
		return nil, domain.ErrSegmentDefinitionInvalid
	}
	if cmd.Definition == nil || len(cmd.Definition) == 0 {
		return nil, domain.ErrSegmentDefinitionInvalid
	}

	if rules, ok := cmd.Definition["rules"]; ok {
		if _, ok := rules.([]any); !ok {
			return nil, domain.ErrSegmentDefinitionInvalid
		}
	}

	id, err := h.idGen()
	if err != nil {
		h.log.Error("failed to generate id", "error", err)
		return nil, err
	}

	segment := domain.Segment{
		ID:             id,
		WorkspaceID:    cmd.WorkspaceID,
		Name:           cmd.Name,
		DefinitionJSON: cmd.Definition,
		Status:         domain.SegmentStatusReady,
		CreatedAt:      cmd.Now,
		UpdatedAt:      cmd.Now,
	}

	if err := h.segmentsWrite.CreateSegment(ctx, segment); err != nil {
		if errors.Is(err, domain.ErrSegmentNameConflict) {
			return nil, err
		}
		h.log.Error("failed to create segment", "error", err)
		return nil, err
	}

	h.log.Info("segment created", "segment_id", id)
	return &segment, nil
}
