package app

import (
	"context"
	"errors"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/ports"
)

type CreateContactInput struct {
	WorkspaceID string
	UserID      string
	Email       string
	FirstName   string
	LastName    string
	Tags        []string
	Attributes  map[string]any
	Now         time.Time
}

type UpdateContactInput struct {
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

func (s *Service) CreateContact(ctx context.Context, input CreateContactInput) (*ContactResult, error) {
	log := s.log.With("usecase", "create_contact", "workspace_id", input.WorkspaceID)
	if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, "audience.write"); err != nil {
		if errors.Is(err, domain.ErrWriteDenied) {
			return nil, err
		}
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, domain.ErrWriteDenied
		}
		return nil, err
	}

	email := input.Email
	if email == "" {
		return nil, domain.ErrContactPayloadInvalid
	}
	normalized := domain.NormalizeEmail(email)

	if normalized == "" {
		return nil, domain.ErrContactPayloadInvalid
	}

	existing, _ := s.contactsRead.FindContactByEmail(ctx, input.WorkspaceID, normalized)
	if existing != nil {
		return nil, domain.ErrContactEmailConflict
	}

	id, err := s.idGen()
	if err != nil {
		log.Error("failed to generate id", "error", err)
		return nil, err
	}

	contact := domain.Contact{
		ID:              id,
		WorkspaceID:     input.WorkspaceID,
		Email:           email,
		EmailNormalized: normalized,
		FirstName:       input.FirstName,
		LastName:        input.LastName,
		Status:          domain.ContactStatusActive,
		Tags:            input.Tags,
		Attributes:      input.Attributes,
		CreatedAt:       input.Now,
		UpdatedAt:       input.Now,
	}

	if err := s.contactsWrite.CreateContact(ctx, contact); err != nil {
		log.Error("failed to create contact", "error", err)
		return nil, err
	}

	log.Info("contact created", "contact_id", id)
	return &ContactResult{Contact: contactToDTO(contact)}, nil
}

func (s *Service) ListContacts(ctx context.Context, query ports.ContactListQuery, userID string) (*ContactListResult, error) {
	log := s.log.With("usecase", "list_contacts", "workspace_id", query.WorkspaceID)
	if err := s.accessChecker.RequirePermission(ctx, query.WorkspaceID, userID, "audience.read"); err != nil {
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, err
		}
		return nil, err
	}

	if query.Limit <= 0 || query.Limit > 100 {
		query.Limit = 50
	}

	contacts, cursor, err := s.contactsRead.ListContacts(ctx, query)
	if err != nil {
		log.Error("failed to list contacts", "error", err)
		return nil, err
	}

	return &ContactListResult{Contacts: contactSliceToDTOs(contacts), NextCursor: cursor}, nil
}

func (s *Service) GetContact(ctx context.Context, workspaceID, contactID, userID string) (*ContactResult, error) {
	log := s.log.With("usecase", "get_contact", "workspace_id", workspaceID, "contact_id", contactID)
	if err := s.accessChecker.RequirePermission(ctx, workspaceID, userID, "audience.read"); err != nil {
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, err
		}
		return nil, err
	}

	contact, err := s.contactsRead.FindContactByID(ctx, workspaceID, contactID)
	if err != nil {
		if errors.Is(err, domain.ErrContactNotFound) {
			return nil, err
		}
		log.Error("failed to find contact", "error", err)
		return nil, err
	}

	return &ContactResult{Contact: contactToDTO(*contact)}, nil
}

func (s *Service) UpdateContact(ctx context.Context, input UpdateContactInput) (*ContactResult, error) {
	log := s.log.With("usecase", "update_contact", "workspace_id", input.WorkspaceID, "contact_id", input.ContactID)
	if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, "audience.write"); err != nil {
		if errors.Is(err, domain.ErrWriteDenied) {
			return nil, err
		}
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, domain.ErrWriteDenied
		}
		return nil, err
	}

	contact, err := s.contactsRead.FindContactByID(ctx, input.WorkspaceID, input.ContactID)
	if err != nil {
		if errors.Is(err, domain.ErrContactNotFound) {
			return nil, err
		}
		log.Error("failed to find contact", "error", err)
		return nil, err
	}

	if input.Email != nil {
		normalized := domain.NormalizeEmail(*input.Email)
		if normalized == "" {
			return nil, domain.ErrContactPayloadInvalid
		}
		existing, _ := s.contactsRead.FindContactByEmail(ctx, input.WorkspaceID, normalized)
		if existing != nil && existing.ID != input.ContactID {
			return nil, domain.ErrContactEmailConflict
		}
		contact.Email = *input.Email
		contact.EmailNormalized = normalized
	}
	if input.FirstName != nil {
		contact.FirstName = *input.FirstName
	}
	if input.LastName != nil {
		contact.LastName = *input.LastName
	}
	if input.Status != nil {
		if !domain.ValidContactStatus(*input.Status) {
			return nil, domain.ErrContactStatusInvalid
		}
		contact.Status = domain.ContactStatus(*input.Status)
	}
	if input.Tags != nil {
		contact.Tags = input.Tags
	}
	if input.Attributes != nil {
		contact.Attributes = input.Attributes
	}
	contact.UpdatedAt = input.Now

	if err := s.contactsWrite.UpdateContact(ctx, *contact); err != nil {
		log.Error("failed to update contact", "error", err)
		return nil, err
	}

	log.Info("contact updated")
	return &ContactResult{Contact: contactToDTO(*contact)}, nil
}

func (s *Service) ArchiveContact(ctx context.Context, workspaceID, contactID, userID string, now time.Time) error {
	log := s.log.With("usecase", "archive_contact", "workspace_id", workspaceID, "contact_id", contactID)
	if err := s.accessChecker.RequirePermission(ctx, workspaceID, userID, "audience.write"); err != nil {
		if errors.Is(err, domain.ErrWriteDenied) {
			return err
		}
		if errors.Is(err, domain.ErrReadDenied) {
			return domain.ErrWriteDenied
		}
		return err
	}

	if _, err := s.contactsRead.FindContactByID(ctx, workspaceID, contactID); err != nil {
		if errors.Is(err, domain.ErrContactNotFound) {
			return err
		}
		log.Error("failed to find contact for archive", "error", err)
		return err
	}

	if err := s.contactsWrite.ArchiveContact(ctx, workspaceID, contactID, now); err != nil {
		log.Error("failed to archive contact", "error", err)
		return err
	}

	log.Info("contact archived")
	return nil
}
