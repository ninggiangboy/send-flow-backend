package resolveaudiencerecipients

import (
	"context"
	"errors"
	"log/slog"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/domain"
	audienceredis "github.com/ninggiangboy/send-flow/backend/internal/modules/audience/infrastructure/redis"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/ports"
)

type Recipient struct {
	ContactID       string
	Email           string
	EmailNormalized string
	FirstName       string
	LastName        string
	Tags            []string
	Attributes      map[string]any
}

type Options struct {
	ContactsRead  ports.ContactReadRepository
	ListsRead     ports.ListReadRepository
	SegmentsRead  ports.SegmentReadRepository
	AccessChecker ports.WorkspaceAccessChecker
	Logger        *slog.Logger
	Cache         *audienceredis.Cache
}

type Command struct {
	WorkspaceID string
	UserID      string
	ListID      string
	SegmentID   string
	ContactIDs  []string
}

type Handler struct {
	contactsRead  ports.ContactReadRepository
	listsRead     ports.ListReadRepository
	segmentsRead  ports.SegmentReadRepository
	accessChecker ports.WorkspaceAccessChecker
	log           *slog.Logger
	cache         *audienceredis.Cache
}

func New(opts Options) *Handler {
	return &Handler{
		contactsRead:  opts.ContactsRead,
		listsRead:     opts.ListsRead,
		segmentsRead:  opts.SegmentsRead,
		accessChecker: opts.AccessChecker,
		log:           opts.Logger.With("usecase", "resolve_audience_recipients"),
		cache:         opts.Cache,
	}
}

func (h *Handler) Execute(ctx context.Context, cmd Command) ([]Recipient, error) {
	if err := h.accessChecker.RequirePermission(ctx, cmd.WorkspaceID, cmd.UserID, "audience:resolve"); err != nil {
		return nil, err
	}

	const maxAudienceRecipients = 100000

	recipients := make(map[string]*Recipient)
	order := make([]string, 0)

	addContact := func(c domain.Contact) bool {
		if _, ok := recipients[c.ID]; !ok {
			if len(order) >= maxAudienceRecipients {
				return false
			}
			recipients[c.ID] = &Recipient{
				ContactID:       c.ID,
				Email:           c.Email,
				EmailNormalized: c.EmailNormalized,
				FirstName:       c.FirstName,
				LastName:        c.LastName,
				Tags:            c.Tags,
				Attributes:      c.Attributes,
			}
			order = append(order, c.ID)
		}
		return true
	}

	if cmd.ListID != "" {
		list, err := h.listsRead.FindListByID(ctx, cmd.WorkspaceID, cmd.ListID)
		if err != nil {
			if errors.Is(err, domain.ErrListNotFound) {
				return nil, err
			}
			h.log.Error("failed to find list", "error", err)
			return nil, err
		}
		_ = list

		query := ports.ContactListQuery{
			WorkspaceID: cmd.WorkspaceID,
			ListID:      cmd.ListID,
			Status:      string(domain.ContactStatusActive),
			Limit:       10000,
		}
		for {
			contacts, cursor, err := h.contactsRead.ListContacts(ctx, query)
			if err != nil {
				h.log.Error("failed to list contacts by list", "error", err)
				return nil, err
			}
			for _, c := range contacts {
				if c.Status == domain.ContactStatusActive {
					if !addContact(c) {
						break
					}
				}
			}
			if cursor == "" || len(order) >= maxAudienceRecipients {
				break
			}
			query.Cursor = cursor
		}
	}

	if cmd.SegmentID != "" {
		segment, err := h.segmentsRead.FindSegmentByID(ctx, cmd.WorkspaceID, cmd.SegmentID)
		if err != nil {
			if errors.Is(err, domain.ErrSegmentNotFound) {
				return nil, err
			}
			h.log.Error("failed to find segment", "error", err)
			return nil, err
		}
		if segment.Status != domain.SegmentStatusReady {
			return nil, domain.ErrSegmentDefinitionInvalid
		}

		rules, err := domain.ExtractRules(segment.DefinitionJSON)
		if err != nil {
			return nil, domain.ErrSegmentDefinitionInvalid
		}

		query := ports.ContactListQuery{
			WorkspaceID: cmd.WorkspaceID,
			Status:      string(domain.ContactStatusActive),
			Limit:       10000,
		}
		for {
			contacts, cursor, err := h.contactsRead.ListContacts(ctx, query)
			if err != nil {
				h.log.Error("failed to list contacts for segment", "error", err)
				return nil, err
			}
			for _, c := range contacts {
				if domain.MatchesSegment(c, rules) {
					if !addContact(c) {
						break
					}
				}
			}
			if cursor == "" || len(order) >= maxAudienceRecipients {
				break
			}
			query.Cursor = cursor
		}
	}

	for _, cid := range cmd.ContactIDs {
		if len(order) >= maxAudienceRecipients {
			break
		}
		contact, err := h.contactsRead.FindContactByID(ctx, cmd.WorkspaceID, cid)
		if err != nil {
			if errors.Is(err, domain.ErrContactNotFound) {
				return nil, err
			}
			h.log.Error("failed to find contact", "error", err)
			return nil, err
		}
		addContact(*contact)
	}

	if len(order) >= maxAudienceRecipients {
		h.log.Warn("audience recipient limit reached, results truncated", "count", len(order), "limit", maxAudienceRecipients)
	}

	result := make([]Recipient, 0, len(order))
	for _, id := range order {
		if r, ok := recipients[id]; ok {
			result = append(result, *r)
		}
	}
	if result == nil {
		result = []Recipient{}
	}
	return result, nil
}
