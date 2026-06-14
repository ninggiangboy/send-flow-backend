package listapikeys

import (
	"context"
	"log/slog"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/access/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/access/ports"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/constants"
)

type Options struct {
	APIKeyRepo    ports.APIKeyRepository
	AccessChecker ports.WorkspaceAccessChecker
	Logger        *slog.Logger
}

type Command struct {
	WorkspaceID string
	ActorUserID string
	Status      string
	Limit       int
	Cursor      string
}

type Handler struct {
	apiKeyRepo    ports.APIKeyRepository
	accessChecker ports.WorkspaceAccessChecker
	log           *slog.Logger
}

func New(opts Options) *Handler {
	return &Handler{
		apiKeyRepo:    opts.APIKeyRepo,
		accessChecker: opts.AccessChecker,
		log:           opts.Logger.With("usecase", "list_api_keys"),
	}
}

func (h *Handler) Execute(ctx context.Context, cmd Command) ([]domain.APIKey, string, error) {
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
