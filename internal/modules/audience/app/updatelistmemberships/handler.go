package updatelistmemberships

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/ports"
)

type Options struct {
	ListsRead     ports.ListReadRepository
	ListsWrite    ports.ListWriteRepository
	AccessChecker ports.WorkspaceAccessChecker
	Logger        *slog.Logger
}

type Command struct {
	WorkspaceID string
	ListID      string
	UserID      string
	Mode        string
	ContactIDs  []string
	Now         time.Time
}

type Handler struct {
	listsRead     ports.ListReadRepository
	listsWrite    ports.ListWriteRepository
	accessChecker ports.WorkspaceAccessChecker
	log           *slog.Logger
}

func New(opts Options) *Handler {
	return &Handler{
		listsRead:     opts.ListsRead,
		listsWrite:    opts.ListsWrite,
		accessChecker: opts.AccessChecker,
		log:           opts.Logger.With("usecase", "update_list_memberships"),
	}
}

func (h *Handler) Execute(ctx context.Context, cmd Command) (*domain.MembershipUpdateResult, error) {
	if err := h.accessChecker.RequirePermission(ctx, cmd.WorkspaceID, cmd.UserID, "audience.write"); err != nil {
		if errors.Is(err, domain.ErrWriteDenied) {
			return nil, err
		}
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, domain.ErrWriteDenied
		}
		return nil, err
	}

	if !domain.ValidListMembershipMode(cmd.Mode) {
		return nil, domain.ErrListMembershipPayloadInvalid
	}

	list, err := h.listsRead.FindListByID(ctx, cmd.WorkspaceID, cmd.ListID)
	if err != nil {
		if errors.Is(err, domain.ErrListNotFound) {
			return nil, err
		}
		h.log.Error("failed to find list", "error", err)
		return nil, err
	}
	_ = list

	deduplicated := make([]string, 0, len(cmd.ContactIDs))
	seen := map[string]struct{}{}
	for _, id := range cmd.ContactIDs {
		if _, ok := seen[id]; !ok {
			seen[id] = struct{}{}
			deduplicated = append(deduplicated, id)
		}
	}

	var result domain.MembershipUpdateResult
	switch domain.ListMembershipMode(cmd.Mode) {
	case domain.ListMembershipModeReplace:
		result, err = h.listsWrite.ReplaceListMemberships(ctx, cmd.WorkspaceID, cmd.ListID, deduplicated, cmd.Now)
	case domain.ListMembershipModeMerge:
		result, err = h.listsWrite.MergeListMemberships(ctx, cmd.WorkspaceID, cmd.ListID, deduplicated, cmd.Now)
	}
	if err != nil {
		h.log.Error("failed to update list memberships", "error", err)
		return nil, err
	}

	h.log.Info("list memberships updated", "list_id", cmd.ListID, "added", result.AddedCount, "removed", result.RemovedCount, "skipped", result.SkippedCount)
	return &result, nil
}
