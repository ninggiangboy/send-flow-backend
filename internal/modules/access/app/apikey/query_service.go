package apikey

import (
	"context"
	"log/slog"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/access/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/access/ports"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/constants"
)

type ListQuery struct {
	WorkspaceID string
	ActorUserID string
	Status      string
	Limit       int
	Cursor      string
}

type ListHandler struct {
	apiKeyRepo    ports.APIKeyRepository
	accessChecker ports.WorkspaceAccessChecker
	log           *slog.Logger
}

func NewListHandler(opts struct {
	APIKeyRepo    ports.APIKeyRepository
	AccessChecker ports.WorkspaceAccessChecker
	Logger        *slog.Logger
}) *ListHandler {
	return &ListHandler{
		apiKeyRepo:    opts.APIKeyRepo,
		accessChecker: opts.AccessChecker,
		log:           opts.Logger.With("usecase", "list_api_keys"),
	}
}

func (h *ListHandler) Execute(ctx context.Context, cmd ListQuery) ([]domain.APIKey, string, error) {
	if err := h.accessChecker.RequirePermission(ctx, cmd.WorkspaceID, cmd.ActorUserID, "api_key.manage"); err != nil {
		return nil, "", err
	}

	limit := cmd.Limit
	if limit <= 0 || limit > 100 {
		limit = constants.DefaultPageSize
	}

	keys, cursor, err := h.apiKeyRepo.ListByWorkspace(ctx, ports.APIKeyListQuery{
		WorkspaceID: cmd.WorkspaceID,
		Status:      cmd.Status,
		Limit:       limit,
		Cursor:      cmd.Cursor,
	})
	if err != nil {
		h.log.Error("failed to list api keys", "error", err, "workspace_id", cmd.WorkspaceID)
		return nil, "", err
	}

	return keys, cursor, nil
}
