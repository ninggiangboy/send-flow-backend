package entry

import (
	"context"
	"log/slog"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/suppression/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/suppression/ports"
)

// --- ListEntries ---

type ListQuery struct {
	WorkspaceID string
	UserID      string
	Query       ports.SuppressionListQuery
}

type ListHandler struct {
	entriesRead   ports.SuppressionReadRepository
	accessChecker ports.WorkspaceAccessChecker
	log           *slog.Logger
}

func NewListHandler(opts struct {
	EntriesRead   ports.SuppressionReadRepository
	AccessChecker ports.WorkspaceAccessChecker
	Logger        *slog.Logger
}) *ListHandler {
	return &ListHandler{
		entriesRead:   opts.EntriesRead,
		accessChecker: opts.AccessChecker,
		log:           opts.Logger.With("usecase", "list_suppression_entries"),
	}
}

func (h *ListHandler) Execute(ctx context.Context, cmd ListQuery) ([]domain.SuppressionEntry, string, error) {
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

// --- CheckSuppression ---

type CheckQuery struct {
	WorkspaceID     string
	EmailNormalized string
	Scope           string
}

type CheckResult struct {
	Suppressed bool
	Reason     string
	Scope      string
	EntryID    string
}

type CheckHandler struct {
	entriesRead ports.SuppressionReadRepository
	log         *slog.Logger
}

func NewCheckHandler(opts struct {
	EntriesRead ports.SuppressionReadRepository
	Logger      *slog.Logger
}) *CheckHandler {
	return &CheckHandler{
		entriesRead: opts.EntriesRead,
		log:         opts.Logger.With("usecase", "check_suppression"),
	}
}

func (h *CheckHandler) Execute(ctx context.Context, cmd CheckQuery) (*CheckResult, error) {
	if cmd.WorkspaceID == "" || cmd.EmailNormalized == "" {
		return &CheckResult{}, nil
	}

	scope := cmd.Scope
	if scope == "" {
		scope = "workspace"
	}

	scopes := []string{scope, "global"}
	entry, err := h.entriesRead.FindActiveByEmail(ctx, ports.SuppressionCheckQuery{
		WorkspaceID:     cmd.WorkspaceID,
		EmailNormalized: cmd.EmailNormalized,
		Scopes:          scopes,
	})
	if err != nil {
		h.log.Error("failed to check suppression", "error", err)
		return nil, err
	}

	if entry == nil {
		return &CheckResult{Suppressed: false}, nil
	}

	return &CheckResult{
		Suppressed: true,
		Reason:     string(entry.Reason),
		Scope:      string(entry.Scope),
		EntryID:    entry.ID,
	}, nil
}
