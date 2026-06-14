package revokeapikey

import (
	"context"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/access/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/access/ports"
)

type Options struct {
	APIKeyRepo    ports.APIKeyRepository
	AccessChecker ports.WorkspaceAccessChecker
	Logger        *slog.Logger
}

type Command struct {
	WorkspaceID string
	ActorUserID string
	APIKeyID    string
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
		log:           opts.Logger.With("usecase", "revoke_api_key"),
	}
}

func (h *Handler) Execute(ctx context.Context, cmd Command) (*domain.APIKey, error) {
	log := h.log.With("workspace_id", cmd.WorkspaceID, "api_key_id", cmd.APIKeyID, "actor_user_id", cmd.ActorUserID)

	if err := h.accessChecker.RequirePermission(ctx, cmd.WorkspaceID, cmd.ActorUserID, "api_key.manage"); err != nil {
		return nil, err
	}

	existing, err := h.apiKeyRepo.FindByID(ctx, cmd.WorkspaceID, cmd.APIKeyID)
	if err != nil {
		return nil, err
	}

	if existing.Status == domain.APIKeyStatusRevoked {
		log.Info("api key already revoked, returning success")
		return existing, nil
	}

	now := time.Now().UTC()
	existing.Status = domain.APIKeyStatusRevoked
	existing.RevokedAt = &now
	existing.UpdatedAt = now

	if err := h.apiKeyRepo.Update(ctx, *existing); err != nil {
		log.Error("failed to revoke api key", "error", err)
		return nil, err
	}

	log.Info("api key revoked")

	return existing, nil
}
