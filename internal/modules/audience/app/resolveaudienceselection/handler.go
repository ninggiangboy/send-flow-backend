package resolveaudienceselection

import (
	"context"
	"errors"
	"log/slog"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/ports"
)

type Options struct {
	ContactsRead  ports.ContactReadRepository
	SegmentsRead  ports.SegmentReadRepository
	AccessChecker ports.WorkspaceAccessChecker
	Logger        *slog.Logger
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
	segmentsRead  ports.SegmentReadRepository
	accessChecker ports.WorkspaceAccessChecker
	log           *slog.Logger
}

func New(opts Options) *Handler {
	return &Handler{
		contactsRead:  opts.ContactsRead,
		segmentsRead:  opts.SegmentsRead,
		accessChecker: opts.AccessChecker,
		log:           opts.Logger.With("usecase", "resolve_audience_selection"),
	}
}

func (h *Handler) Execute(ctx context.Context, cmd Command) ([]string, error) {
	if err := h.accessChecker.RequirePermission(ctx, cmd.WorkspaceID, cmd.UserID, "audience:resolve"); err != nil {
		return nil, err
	}

	contactIDs := make(map[string]struct{})

	if cmd.ListID != "" {
		query := ports.ContactListQuery{
			WorkspaceID: cmd.WorkspaceID,
			ListID:      cmd.ListID,
			Limit:       10000,
		}
		for {
			contacts, cursor, err := h.contactsRead.ListContacts(ctx, query)
			if err != nil {
				h.log.Error("failed to list contacts by list", "error", err)
				return nil, err
			}
			for _, c := range contacts {
				contactIDs[c.ID] = struct{}{}
			}
			if cursor == "" {
				break
			}
			query.Cursor = cursor
		}
	}

	if cmd.SegmentID != "" {
		_, err := h.segmentsRead.FindSegmentByID(ctx, cmd.WorkspaceID, cmd.SegmentID)
		if err != nil {
			if errors.Is(err, domain.ErrSegmentNotFound) {
				return nil, err
			}
			h.log.Error("failed to find segment", "error", err)
			return nil, err
		}
	}

	for _, id := range cmd.ContactIDs {
		contactIDs[id] = struct{}{}
	}

	result := make([]string, 0, len(contactIDs))
	for id := range contactIDs {
		result = append(result, id)
	}
	if result == nil {
		result = []string{}
	}
	return result, nil
}
