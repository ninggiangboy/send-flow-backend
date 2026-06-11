package app

import (
	"context"
	"errors"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/ports"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/constants"
)

func (s *Service) CreateList(ctx context.Context, workspaceID, userID, name, description string, metadata map[string]any, now time.Time) (*ListResult, error) {
	log := s.log.With("usecase", "create_list", "workspace_id", workspaceID)
	if err := s.accessChecker.RequirePermission(ctx, workspaceID, userID, "audience.write"); err != nil {
		if errors.Is(err, domain.ErrWriteDenied) {
			return nil, err
		}
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, domain.ErrWriteDenied
		}
		return nil, err
	}

	if name == "" {
		return nil, domain.ErrContactPayloadInvalid
	}

	id, err := s.idGen()
	if err != nil {
		log.Error("failed to generate id", "error", err)
		return nil, err
	}

	list := domain.AudienceList{
		ID:          id,
		WorkspaceID: workspaceID,
		Name:        name,
		Description: description,
		Metadata:    metadata,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := s.listsWrite.CreateList(ctx, list); err != nil {
		if errors.Is(err, domain.ErrListNameConflict) {
			return nil, err
		}
		log.Error("failed to create list", "error", err)
		return nil, err
	}

	log.Info("list created", "list_id", id)
	return &ListResult{List: listToDTO(list)}, nil
}

func (s *Service) ListLists(ctx context.Context, workspaceID, userID string, limit int, cursor string) (*ListListResult, error) {
	log := s.log.With("usecase", "list_lists", "workspace_id", workspaceID)
	if err := s.accessChecker.RequirePermission(ctx, workspaceID, userID, "audience.read"); err != nil {
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, err
		}
		return nil, err
	}

	if limit <= 0 || limit > 100 {
		limit = constants.DefaultPageSize
	}

	query := ports.ListListQuery{WorkspaceID: workspaceID, Limit: limit, Cursor: cursor}
	lists, nextCursor, err := s.listsRead.ListLists(ctx, query)
	if err != nil {
		log.Error("failed to list lists", "error", err)
		return nil, err
	}

	listIDs := make([]string, len(lists))
	for i, l := range lists {
		listIDs[i] = l.ID
	}

	counts := map[string]int64{}
	if len(listIDs) > 0 {
		var countErr error
		counts, countErr = s.listsRead.CountContactsByList(ctx, workspaceID, listIDs)
		if countErr != nil {
			log.Error("failed to count contacts by list", "error", countErr)
		}
	}

	return &ListListResult{Lists: listSliceToDTOs(lists), Counts: counts, NextCursor: nextCursor}, nil
}

func (s *Service) UpdateListMemberships(ctx context.Context, workspaceID, listID, userID, mode string, contactIDs []string, now time.Time) (*MembershipUpdateResult, error) {
	log := s.log.With("usecase", "update_list_memberships", "workspace_id", workspaceID, "list_id", listID)
	if err := s.accessChecker.RequirePermission(ctx, workspaceID, userID, "audience.write"); err != nil {
		if errors.Is(err, domain.ErrWriteDenied) {
			return nil, err
		}
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, domain.ErrWriteDenied
		}
		return nil, err
	}

	if !domain.ValidListMembershipMode(mode) {
		return nil, domain.ErrListMembershipPayloadInvalid
	}

	list, err := s.listsRead.FindListByID(ctx, workspaceID, listID)
	if err != nil {
		if errors.Is(err, domain.ErrListNotFound) {
			return nil, err
		}
		log.Error("failed to find list", "error", err)
		return nil, err
	}
	_ = list

	deduplicated := make([]string, 0, len(contactIDs))
	seen := map[string]struct{}{}
	for _, id := range contactIDs {
		if _, ok := seen[id]; !ok {
			seen[id] = struct{}{}
			deduplicated = append(deduplicated, id)
		}
	}

	var result domain.MembershipUpdateResult
	switch domain.ListMembershipMode(mode) {
	case domain.ListMembershipModeReplace:
		result, err = s.listsWrite.ReplaceListMemberships(ctx, workspaceID, listID, deduplicated, now)
	case domain.ListMembershipModeMerge:
		result, err = s.listsWrite.MergeListMemberships(ctx, workspaceID, listID, deduplicated, now)
	}
	if err != nil {
		log.Error("failed to update list memberships", "error", err)
		return nil, err
	}

	log.Info("list memberships updated", "list_id", listID, "added", result.AddedCount, "removed", result.RemovedCount, "skipped", result.SkippedCount)
	return &MembershipUpdateResult{Result: MembershipUpdateResultDTO{AddedCount: result.AddedCount, RemovedCount: result.RemovedCount, SkippedCount: result.SkippedCount}}, nil
}
