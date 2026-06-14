package listsegments

import (
	"context"
	"errors"
	"log/slog"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/ports"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/constants"
)

type Options struct {
	SegmentsRead  ports.SegmentReadRepository
	AccessChecker ports.WorkspaceAccessChecker
	Logger        *slog.Logger
}

type Command struct {
	WorkspaceID string
	UserID      string
	Status      string
	Limit       int
	Cursor      string
}

type Handler struct {
	segmentsRead  ports.SegmentReadRepository
	accessChecker ports.WorkspaceAccessChecker
	log           *slog.Logger
}

func New(opts Options) *Handler {
	return &Handler{
		segmentsRead:  opts.SegmentsRead,
		accessChecker: opts.AccessChecker,
		log:           opts.Logger.With("usecase", "list_segments"),
	}
}

func (h *Handler) Execute(ctx context.Context, cmd Command) ([]domain.Segment, string, error) {
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

	query := ports.SegmentListQuery{WorkspaceID: cmd.WorkspaceID, Status: cmd.Status, Limit: limit, Cursor: cmd.Cursor}
	segments, nextCursor, err := h.segmentsRead.ListSegments(ctx, query)
	if err != nil {
		h.log.Error("failed to list segments", "error", err)
		return nil, "", err
	}

	return segments, nextCursor, nil
}
