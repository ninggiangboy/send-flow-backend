package getcontact

import (
	"context"
	"errors"
	"log/slog"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/ports"
)

type Options struct {
	ContactsRead  ports.ContactReadRepository
	AccessChecker ports.WorkspaceAccessChecker
	Logger        *slog.Logger
}

type Command struct {
	WorkspaceID string
	ContactID   string
	UserID      string
}

type Handler struct {
	contactsRead  ports.ContactReadRepository
	accessChecker ports.WorkspaceAccessChecker
	log           *slog.Logger
}

func New(opts Options) *Handler {
	return &Handler{
		contactsRead:  opts.ContactsRead,
		accessChecker: opts.AccessChecker,
		log:           opts.Logger.With("usecase", "get_contact"),
	}
}

func (h *Handler) Execute(ctx context.Context, cmd Command) (*domain.Contact, error) {
	if err := h.accessChecker.RequirePermission(ctx, cmd.WorkspaceID, cmd.UserID, "audience.read"); err != nil {
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, err
		}
		return nil, err
	}

	contact, err := h.contactsRead.FindContactByID(ctx, cmd.WorkspaceID, cmd.ContactID)
	if err != nil {
		if errors.Is(err, domain.ErrContactNotFound) {
			return nil, err
		}
		h.log.Error("failed to find contact", "error", err)
		return nil, err
	}

	return contact, nil
}
