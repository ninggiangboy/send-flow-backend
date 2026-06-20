package apikey

import (
	"context"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/access/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/access/ports"
)

type CreateInput struct {
	WorkspaceID string
	ActorUserID string
	Name        string
	Scopes      []string
	ExpiresAt   *time.Time
	QuotaLimits *domain.EmailQuotaLimits
}

type CreateResult struct {
	APIKey domain.APIKey
	Secret string
}

type CreateHandler struct {
	apiKeyRepo    ports.APIKeyRepository
	accessChecker ports.WorkspaceAccessChecker
	idGen         ports.IDGenerator
	secretGen     ports.SecretGenerator
	secretHasher  ports.SecretHasher
	log           *slog.Logger
}

func NewCreateHandler(opts struct {
	APIKeyRepo    ports.APIKeyRepository
	AccessChecker ports.WorkspaceAccessChecker
	IDGen         ports.IDGenerator
	SecretGen     ports.SecretGenerator
	SecretHasher  ports.SecretHasher
	Logger        *slog.Logger
}) *CreateHandler {
	return &CreateHandler{
		apiKeyRepo:    opts.APIKeyRepo,
		accessChecker: opts.AccessChecker,
		idGen:         opts.IDGen,
		secretGen:     opts.SecretGen,
		secretHasher:  opts.SecretHasher,
		log:           opts.Logger.With("usecase", "create_api_key"),
	}
}

func (h *CreateHandler) Execute(ctx context.Context, cmd CreateInput) (*CreateResult, error) {
	log := h.log.With("workspace_id", cmd.WorkspaceID, "actor_user_id", cmd.ActorUserID)

	if cmd.WorkspaceID == "" || cmd.ActorUserID == "" {
		return nil, domain.ErrPayloadInvalid
	}

	if err := h.accessChecker.RequirePermission(ctx, cmd.WorkspaceID, cmd.ActorUserID, "api_key.manage"); err != nil {
		return nil, err
	}

	name := domain.NormalizeName(cmd.Name)
	if name == "" || len(name) > 120 {
		return nil, domain.ErrAPIKeyConfigInvalid
	}

	if err := domain.ValidateScopes(cmd.Scopes); err != nil {
		log.Warn("invalid scopes for create", "scopes", cmd.Scopes)
		return nil, err
	}

	if cmd.QuotaLimits != nil {
		if err := cmd.QuotaLimits.Validate(); err != nil {
			log.Warn("invalid quota limits for create", "error", err)
			return nil, err
		}
	}

	now := time.Now().UTC()

	if cmd.ExpiresAt != nil && !cmd.ExpiresAt.IsZero() && cmd.ExpiresAt.Before(now) {
		return nil, domain.ErrAPIKeyConfigInvalid
	}

	id, err := h.idGen()
	if err != nil {
		log.Error("failed to generate id", "error", err)
		return nil, err
	}

	plaintext, keyPrefix, err := h.secretGen.Generate()
	if err != nil {
		log.Error("failed to generate secret", "error", err)
		return nil, err
	}

	secretHash, err := h.secretHasher.Hash(plaintext)
	if err != nil {
		log.Error("failed to hash secret", "error", err)
		return nil, err
	}

	key := domain.APIKey{
		ID:          id,
		WorkspaceID: cmd.WorkspaceID,
		Name:        name,
		KeyPrefix:   keyPrefix,
		SecretHash:  secretHash,
		Scopes:      cmd.Scopes,
		Status:      domain.APIKeyStatusActive,
		CreatedAt:   now,
		UpdatedAt:   now,
		ExpiresAt:   cmd.ExpiresAt,
		QuotaLimits: cmd.QuotaLimits,
	}

	if err := h.apiKeyRepo.Create(ctx, key); err != nil {
		log.Error("failed to persist api key", "error", err)
		return nil, err
	}

	log.Info("api key created", "api_key_id", id)

	return &CreateResult{APIKey: key, Secret: plaintext}, nil
}
