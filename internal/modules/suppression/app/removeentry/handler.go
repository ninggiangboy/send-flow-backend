package removeentry

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/suppression/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/suppression/ports"
)

type Options struct {
	EntriesRead   ports.SuppressionReadRepository
	EntriesWrite  ports.SuppressionWriteRepository
	AccessChecker ports.WorkspaceAccessChecker
	Logger        *slog.Logger
}

type Command struct {
	WorkspaceID string
	EntryID     string
	UserID      string
	Now         time.Time
}

type Handler struct {
	entriesRead   ports.SuppressionReadRepository
	entriesWrite  ports.SuppressionWriteRepository
	accessChecker ports.WorkspaceAccessChecker
	log           *slog.Logger
}

func New(opts Options) *Handler {
	return &Handler{
		entriesRead:   opts.EntriesRead,
		entriesWrite:  opts.EntriesWrite,
		accessChecker: opts.AccessChecker,
		log:           opts.Logger.With("usecase", "remove_suppression_entry"),
	}
}

func (h *Handler) Execute(ctx context.Context, cmd Command) error {
	if err := h.accessChecker.RequirePermission(ctx, cmd.WorkspaceID, cmd.UserID, "suppression.manage"); err != nil {
		if errors.Is(err, domain.ErrManageDenied) {
			return err
		}
		if errors.Is(err, domain.ErrReadDenied) {
			return domain.ErrManageDenied
		}
		return err
	}

	entry, err := h.entriesRead.FindByID(ctx, cmd.WorkspaceID, cmd.EntryID)
	if err != nil {
		if errors.Is(err, domain.ErrEntryNotFound) {
			return err
		}
		h.log.Error("failed to find entry for removal", "error", err)
		return err
	}

	if entry.Status == domain.SuppressionStatusRemoved {
		return domain.ErrUnsuppressConflict
	}

	if err := h.entriesWrite.Remove(ctx, cmd.WorkspaceID, cmd.EntryID, cmd.Now); err != nil {
		h.log.Error("failed to remove suppression entry", "error", err)
		return err
	}

	h.log.Info("suppression entry removed")
	return nil
}
