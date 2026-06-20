package apikey

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/access/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/access/ports"
)

type AuthenticateInput struct {
	BearerToken string
}

type AuthenticateResult struct {
	WorkspaceID string
	APIKeyID    string
	Scopes      []string
	KeyPrefix   string
}

type AuthenticateHandler struct {
	apiKeyRepo   ports.APIKeyRepository
	secretHasher ports.SecretHasher
	log          *slog.Logger
}

func NewAuthenticateHandler(opts struct {
	APIKeyRepo   ports.APIKeyRepository
	SecretHasher ports.SecretHasher
	Logger       *slog.Logger
}) *AuthenticateHandler {
	return &AuthenticateHandler{
		apiKeyRepo:   opts.APIKeyRepo,
		secretHasher: opts.SecretHasher,
		log:          opts.Logger.With("usecase", "authenticate_api_key"),
	}
}

func (h *AuthenticateHandler) Execute(ctx context.Context, input AuthenticateInput) (*AuthenticateResult, error) {
	if input.BearerToken == "" {
		return nil, domain.ErrAPIKeyInvalid
	}

	prefix := domain.DerivePrefix(input.BearerToken)

	key, err := h.apiKeyRepo.FindByPrefix(ctx, prefix)
	if err != nil {
		if errors.Is(err, domain.ErrAPIKeyNotFound) {
			return nil, domain.ErrAPIKeyInvalid
		}
		h.log.Error("failed to find api key by prefix", "error", err)
		return nil, domain.ErrAPIKeyInvalid
	}

	if !h.secretHasher.Verify(key.SecretHash, input.BearerToken) {
		return nil, domain.ErrAPIKeyInvalid
	}

	if !domain.IsActive(*key, time.Now().UTC()) {
		return nil, domain.ErrAPIKeyInvalid
	}

	if err := h.apiKeyRepo.TouchLastUsed(ctx, key.WorkspaceID, key.ID, time.Now().UTC()); err != nil {
		h.log.Error("failed to touch last_used_at", "error", err, "api_key_id", key.ID)
	}

	return &AuthenticateResult{
		WorkspaceID: key.WorkspaceID,
		APIKeyID:    key.ID,
		Scopes:      key.Scopes,
		KeyPrefix:   key.KeyPrefix,
	}, nil
}
