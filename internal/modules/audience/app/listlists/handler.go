package listlists

import (
	"context"
	"errors"
	"log/slog"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/ports"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/constants"
)

type Options struct {
	ListsRead     ports.ListReadRepository
	AccessChecker ports.WorkspaceAccessChecker
	Logger        *slog.Logger
}

type Command struct {
	WorkspaceID string
	UserID      string
	Limit       int
	Cursor      string
}

type ListCounts struct {
	Lists      []domain.AudienceList
	Counts     map[string]int64
	NextCursor string
}

type Handler struct {
	listsRead     ports.ListReadRepository
	accessChecker ports.WorkspaceAccessChecker
	log           *slog.Logger
}

func New(opts Options) *Handler {
	return &Handler{
		listsRead:     opts.ListsRead,
		accessChecker: opts.AccessChecker,
		log:           opts.Logger.With("usecase", "list_lists"),
	}
}

func (h *Handler) Execute(ctx context.Context, cmd Command) (ListCounts, error) {
	if err := h.accessChecker.RequirePermission(ctx, cmd.WorkspaceID, cmd.UserID, "audience.read"); err != nil {
		if errors.Is(err, domain.ErrReadDenied) {
			return ListCounts{}, err
		}
		return ListCounts{}, err
	}

	limit := cmd.Limit
	if limit <= 0 || limit > 100 {
		limit = constants.DefaultPageSize
	}

	query := ports.ListListQuery{WorkspaceID: cmd.WorkspaceID, Limit: limit, Cursor: cmd.Cursor}
	lists, nextCursor, err := h.listsRead.ListLists(ctx, query)
	if err != nil {
		h.log.Error("failed to list lists", "error", err)
		return ListCounts{}, err
	}

	listIDs := make([]string, len(lists))
	for i, l := range lists {
		listIDs[i] = l.ID
	}

	counts := map[string]int64{}
	if len(listIDs) > 0 {
		var countErr error
		counts, countErr = h.listsRead.CountContactsByList(ctx, cmd.WorkspaceID, listIDs)
		if countErr != nil {
			h.log.Error("failed to count contacts by list", "error", countErr)
		}
	}

	return ListCounts{Lists: lists, Counts: counts, NextCursor: nextCursor}, nil
}
