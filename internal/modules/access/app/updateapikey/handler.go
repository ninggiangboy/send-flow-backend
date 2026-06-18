package updateapikey

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
	SecretGen     ports.SecretGenerator
	SecretHasher  ports.SecretHasher
	Logger        *slog.Logger
}

type Command struct {
	WorkspaceID string
	ActorUserID string
	APIKeyID    string
	Name        *string
	Scopes      []string
	ExpiresAt   *time.Time
	Rotate      bool
	QuotaLimits *domain.EmailQuotaLimits
}

type Result struct {
	APIKey domain.APIKey
	Secret string
}

type Handler struct {
	apiKeyRepo    ports.APIKeyRepository
	accessChecker ports.WorkspaceAccessChecker
	secretGen     ports.SecretGenerator
	secretHasher  ports.SecretHasher
	log           *slog.Logger
}

func New(opts Options) *Handler {
	return &Handler{
		apiKeyRepo:    opts.APIKeyRepo,
		accessChecker: opts.AccessChecker,
		secretGen:     opts.SecretGen,
		secretHasher:  opts.SecretHasher,
		log:           opts.Logger.With("usecase", "update_api_key"),
	}
}

func (h *Handler) Execute(ctx context.Context, cmd Command) (*Result, error) {
	log := h.log.With("workspace_id", cmd.WorkspaceID, "api_key_id", cmd.APIKeyID, "actor_user_id", cmd.ActorUserID)

	if err := h.accessChecker.RequirePermission(ctx, cmd.WorkspaceID, cmd.ActorUserID, "api_key.manage"); err != nil {
		return nil, err
	}

	existing, err := h.apiKeyRepo.FindByID(ctx, cmd.WorkspaceID, cmd.APIKeyID)
	if err != nil {
		return nil, err
	}

	if existing.Status != domain.APIKeyStatusActive {
		log.Warn("attempted to update non-active key", "status", existing.Status)
		return nil, domain.ErrAPIKeyRotateConflict
	}

	now := time.Now().UTC()

	if cmd.Name != nil {
		name := domain.NormalizeName(*cmd.Name)
		if name == "" || len(name) > 120 {
			return nil, domain.ErrAPIKeyConfigInvalid
		}
		existing.Name = name
	}

	if cmd.Scopes != nil {
		if err := domain.ValidateScopes(cmd.Scopes); err != nil {
			log.Warn("invalid scopes for update", "scopes", cmd.Scopes)
			return nil, err
		}
		existing.Scopes = cmd.Scopes
	}

	if cmd.ExpiresAt != nil {
		if !cmd.ExpiresAt.IsZero() && cmd.ExpiresAt.Before(now) {
			return nil, domain.ErrAPIKeyConfigInvalid
		}
		existing.ExpiresAt = cmd.ExpiresAt
	}

	if cmd.QuotaLimits != nil {
		if err := cmd.QuotaLimits.Validate(); err != nil {
			log.Warn("invalid quota limits for update", "error", err)
			return nil, err
		}
		if cmd.QuotaLimits.IsEmpty() {
			existing.QuotaLimits = nil
		} else {
			existing.QuotaLimits = cmd.QuotaLimits
		}
	}

	var newSecret string

	if cmd.Rotate {
		plaintext, keyPrefix, err := h.secretGen.Generate()
		if err != nil {
			log.Error("failed to generate secret for rotation", "error", err)
			return nil, err
		}

		secretHash, err := h.secretHasher.Hash(plaintext)
		if err != nil {
			log.Error("failed to hash secret for rotation", "error", err)
			return nil, err
		}

		existing.KeyPrefix = keyPrefix
		existing.SecretHash = secretHash
		newSecret = plaintext
	}

	existing.UpdatedAt = now

	if err := h.apiKeyRepo.Update(ctx, *existing); err != nil {
		log.Error("failed to update api key", "error", err)
		return nil, err
	}

	log.Info("api key updated", "rotated", cmd.Rotate)

	return &Result{APIKey: *existing, Secret: newSecret}, nil
}
