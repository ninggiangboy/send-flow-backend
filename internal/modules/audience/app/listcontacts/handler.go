package listcontacts

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
	Query  ports.ContactListQuery
	UserID string
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
		log:           opts.Logger.With("usecase", "list_contacts"),
	}
}

func (h *Handler) Execute(ctx context.Context, cmd Command) ([]domain.Contact, string, error) {
	if err := h.accessChecker.RequirePermission(ctx, cmd.Query.WorkspaceID, cmd.UserID, "audience.read"); err != nil {
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, "", err
		}
		return nil, "", err
	}

	query := cmd.Query
	if query.Limit <= 0 || query.Limit > 100 {
		query.Limit = 50
	}

	contacts, cursor, err := h.contactsRead.ListContacts(ctx, query)
	if err != nil {
		h.log.Error("failed to list contacts", "error", err)
		return nil, "", err
	}

	return contacts, cursor, nil
}
