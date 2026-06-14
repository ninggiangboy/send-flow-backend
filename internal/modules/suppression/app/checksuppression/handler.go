package checksuppression

import (
	"context"
	"log/slog"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/suppression/ports"
)

type Options struct {
	EntriesRead ports.SuppressionReadRepository
	Logger      *slog.Logger
}

type Command struct {
	WorkspaceID     string
	EmailNormalized string
	Scope           string
}

type Result struct {
	Suppressed bool
	Reason     string
	Scope      string
	EntryID    string
}

type Handler struct {
	entriesRead ports.SuppressionReadRepository
	log         *slog.Logger
}

func New(opts Options) *Handler {
	return &Handler{
		entriesRead: opts.EntriesRead,
		log:         opts.Logger.With("usecase", "check_suppression"),
	}
}

func (h *Handler) Execute(ctx context.Context, cmd Command) (*Result, error) {
	if cmd.WorkspaceID == "" || cmd.EmailNormalized == "" {
		return &Result{}, nil
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
		return &Result{Suppressed: false}, nil
	}

	return &Result{
		Suppressed: true,
		Reason:     string(entry.Reason),
		Scope:      string(entry.Scope),
		EntryID:    entry.ID,
	}, nil
}
