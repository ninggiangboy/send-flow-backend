package listentries

import (
	"context"
	"log/slog"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/suppression/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/suppression/ports"
)

type Options struct {
	EntriesRead   ports.SuppressionReadRepository
	AccessChecker ports.WorkspaceAccessChecker
	Logger        *slog.Logger
}

type Command struct {
	WorkspaceID string
	UserID      string
	Query       ports.SuppressionListQuery
}

type Handler struct {
	entriesRead   ports.SuppressionReadRepository
	accessChecker ports.WorkspaceAccessChecker
	log           *slog.Logger
}

func New(opts Options) *Handler {
	return &Handler{
		entriesRead:   opts.EntriesRead,
		accessChecker: opts.AccessChecker,
		log:           opts.Logger.With("usecase", "list_suppression_entries"),
	}
}

func (h *Handler) Execute(ctx context.Context, cmd Command) ([]domain.SuppressionEntry, string, error) {
	if err := h.accessChecker.RequirePermission(ctx, cmd.Query.WorkspaceID, cmd.UserID, "suppression.read"); err != nil {
		return nil, "", err
	}

	query := cmd.Query
	if query.Limit <= 0 || query.Limit > 100 {
		query.Limit = 50
	}

	entries, cursor, err := h.entriesRead.List(ctx, query)
	if err != nil {
		h.log.Error("failed to list suppression entries", "error", err)
		return nil, "", err
	}

	return entries, cursor, nil
}
