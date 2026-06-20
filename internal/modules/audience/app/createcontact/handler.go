package createcontact

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/ports"
)

type Options struct {
	ContactsWrite ports.ContactWriteRepository
	AccessChecker ports.WorkspaceAccessChecker
	IDGen         func() (string, error)
	Logger        *slog.Logger
}

type Command struct {
	WorkspaceID string
	UserID      string
	Email       string
	FirstName   string
	LastName    string
	Tags        []string
	Attributes  map[string]any
	Now         time.Time
}

type Handler struct {
	contactsWrite ports.ContactWriteRepository
	accessChecker ports.WorkspaceAccessChecker
	idGen         func() (string, error)
	log           *slog.Logger
}

func New(opts Options) *Handler {
	return &Handler{
		contactsWrite: opts.ContactsWrite,
		accessChecker: opts.AccessChecker,
		idGen:         opts.IDGen,
		log:           opts.Logger.With("usecase", "create_contact"),
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

	email := cmd.Email
	if email == "" {
		return nil, domain.ErrContactPayloadInvalid
	}
	normalized := domain.NormalizeEmail(email)

	if normalized == "" {
		return nil, domain.ErrContactPayloadInvalid
	}

	existing, _ := h.contactsWrite.FindContactByEmail(ctx, cmd.WorkspaceID, normalized)
	if existing != nil {
		return nil, domain.ErrContactEmailConflict
	}

	id, err := h.idGen()
	if err != nil {
		h.log.Error("failed to generate id", "error", err)
		return nil, err
	}

	contact := domain.Contact{
		ID:              id,
		WorkspaceID:     cmd.WorkspaceID,
		Email:           email,
		EmailNormalized: normalized,
		FirstName:       cmd.FirstName,
		LastName:        cmd.LastName,
		Status:          domain.ContactStatusActive,
		Tags:            cmd.Tags,
		Attributes:      cmd.Attributes,
		CreatedAt:       cmd.Now,
		UpdatedAt:       cmd.Now,
	}

	if err := h.contactsWrite.CreateContact(ctx, contact); err != nil {
		h.log.Error("failed to create contact", "error", err)
		return nil, err
	}

	h.log.Info("contact created", "contact_id", id)
	return &contact, nil
}
