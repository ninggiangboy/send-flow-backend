package contact

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/ports"
)

type ArchiveOptions struct {
	ContactsWrite ports.ContactWriteRepository
	AccessChecker ports.WorkspaceAccessChecker
	Logger        *slog.Logger
}

type ArchiveCommand struct {
	WorkspaceID string
	ContactID   string
	UserID      string
	Now         time.Time
}

type ArchiveHandler struct {
	contactsWrite ports.ContactWriteRepository
	accessChecker ports.WorkspaceAccessChecker
	log           *slog.Logger
}

func NewArchive(opts ArchiveOptions) *ArchiveHandler {
	return &ArchiveHandler{
		contactsWrite: opts.ContactsWrite,
		accessChecker: opts.AccessChecker,
		log:           opts.Logger.With("usecase", "archive_contact"),
	}
}

func (h *ArchiveHandler) Execute(ctx context.Context, cmd ArchiveCommand) error {
	if err := h.accessChecker.RequirePermission(ctx, cmd.WorkspaceID, cmd.UserID, "audience.write"); err != nil {
		if errors.Is(err, domain.ErrWriteDenied) {
			return err
		}
		if errors.Is(err, domain.ErrReadDenied) {
			return domain.ErrWriteDenied
		}
		return err
	}

	if _, err := h.contactsWrite.FindContactByID(ctx, cmd.WorkspaceID, cmd.ContactID); err != nil {
		if errors.Is(err, domain.ErrContactNotFound) {
			return err
		}
		h.log.Error("failed to find contact for archive", "error", err)
		return err
	}

	if err := h.contactsWrite.ArchiveContact(ctx, cmd.WorkspaceID, cmd.ContactID, cmd.Now); err != nil {
		h.log.Error("failed to archive contact", "error", err)
		return err
	}

	h.log.Info("contact archived")
	return nil
}
