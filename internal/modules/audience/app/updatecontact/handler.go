package updatecontact

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/ports"
)

type Options struct {
	ContactsRead  ports.ContactReadRepository
	ContactsWrite ports.ContactWriteRepository
	AccessChecker ports.WorkspaceAccessChecker
	Logger        *slog.Logger
}

type Command struct {
	WorkspaceID string
	ContactID   string
	UserID      string
	Email       *string
	FirstName   *string
	LastName    *string
	Status      *string
	Tags        []string
	Attributes  map[string]any
	Now         time.Time
}

type Handler struct {
	contactsRead  ports.ContactReadRepository
	contactsWrite ports.ContactWriteRepository
	accessChecker ports.WorkspaceAccessChecker
	log           *slog.Logger
}

func New(opts Options) *Handler {
	return &Handler{
		contactsRead:  opts.ContactsRead,
		contactsWrite: opts.ContactsWrite,
		accessChecker: opts.AccessChecker,
		log:           opts.Logger.With("usecase", "update_contact"),
	}
}

func (h *Handler) Execute(ctx context.Context, cmd Command) (*domain.Contact, error) {
	if err := h.accessChecker.RequirePermission(ctx, cmd.WorkspaceID, cmd.UserID, "audience.write"); err != nil {
		if errors.Is(err, domain.ErrWriteDenied) {
			return nil, err
		}
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, domain.ErrWriteDenied
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

	if cmd.Email != nil {
		normalized := domain.NormalizeEmail(*cmd.Email)
		if normalized == "" {
			return nil, domain.ErrContactPayloadInvalid
		}
		existing, _ := h.contactsRead.FindContactByEmail(ctx, cmd.WorkspaceID, normalized)
		if existing != nil && existing.ID != cmd.ContactID {
			return nil, domain.ErrContactEmailConflict
		}
		contact.Email = *cmd.Email
		contact.EmailNormalized = normalized
	}
	if cmd.FirstName != nil {
		contact.FirstName = *cmd.FirstName
	}
	if cmd.LastName != nil {
		contact.LastName = *cmd.LastName
	}
	if cmd.Status != nil {
		if !domain.ValidContactStatus(*cmd.Status) {
			return nil, domain.ErrContactStatusInvalid
		}
		contact.Status = domain.ContactStatus(*cmd.Status)
	}
	if cmd.Tags != nil {
		contact.Tags = cmd.Tags
	}
	if cmd.Attributes != nil {
		contact.Attributes = cmd.Attributes
	}
	contact.UpdatedAt = cmd.Now

	if err := h.contactsWrite.UpdateContact(ctx, *contact); err != nil {
		h.log.Error("failed to update contact", "error", err)
		return nil, err
	}

	h.log.Info("contact updated")
	return contact, nil
}
