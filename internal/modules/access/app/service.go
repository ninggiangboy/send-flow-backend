package app

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/access/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/access/ports"
)

type Options struct {
	APIKeyRepo    ports.APIKeyRepository
	AccessChecker ports.WorkspaceAccessChecker
	IDGen         ports.IDGenerator
	SecretGen     ports.SecretGenerator
	SecretHasher  ports.SecretHasher
	Logger        *slog.Logger
}

type Service struct {
	apiKeyRepo    ports.APIKeyRepository
	accessChecker ports.WorkspaceAccessChecker
	idGen         ports.IDGenerator
	secretGen     ports.SecretGenerator
	secretHasher  ports.SecretHasher
	logger        *slog.Logger
}

func NewService(opts Options) *Service {
	return &Service{
		apiKeyRepo:    opts.APIKeyRepo,
		accessChecker: opts.AccessChecker,
		idGen:         opts.IDGen,
		secretGen:     opts.SecretGen,
		secretHasher:  opts.SecretHasher,
		logger:        opts.Logger.With("module", "access", "service", "api_key"),
	}
}

type ListAPIKeysInput struct {
	WorkspaceID string
	ActorUserID string
	Status      string
	Limit       int
	Cursor      string
}

type ListAPIKeysResult struct {
	APIKeys []APIKeyResult
	Cursor  string
}

type APIKeyResult struct {
	ID          string     `json:"id"`
	WorkspaceID string     `json:"workspace_id"`
	Name        string     `json:"name"`
	KeyPrefix   string     `json:"key_prefix"`
	Scopes      []string   `json:"scopes"`
	Status      string     `json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	LastUsedAt  *time.Time `json:"last_used_at"`
	ExpiresAt   *time.Time `json:"expires_at"`
	RevokedAt   *time.Time `json:"revoked_at"`
}

type CreateAPIKeyInput struct {
	WorkspaceID string
	ActorUserID string
	Name        string
	Scopes      []string
	ExpiresAt   *time.Time
}

type CreateAPIKeyResult struct {
	APIKeyResult
	Secret string `json:"secret"`
}

type UpdateAPIKeyInput struct {
	WorkspaceID string
	ActorUserID string
	APIKeyID    string
	Name        *string
	Scopes      []string
	ExpiresAt   *time.Time
	Rotate      bool
}

type UpdateAPIKeyResult struct {
	APIKeyResult
	Secret string `json:"secret,omitempty"`
}

type RevokeAPIKeyInput struct {
	WorkspaceID string
	ActorUserID string
	APIKeyID    string
}

type AuthenticateAPIKeyInput struct {
	BearerToken string
}

type AuthenticatedAPIKey struct {
	WorkspaceID string
	APIKeyID    string
	Scopes      []string
	KeyPrefix   string
}

func (s *Service) ListAPIKeys(ctx context.Context, input ListAPIKeysInput) (*ListAPIKeysResult, error) {
	if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.ActorUserID, "api_key.manage"); err != nil {
		return nil, err
	}

	limit := input.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	keys, cursor, err := s.apiKeyRepo.ListByWorkspace(ctx, ports.APIKeyListQuery{
		WorkspaceID: input.WorkspaceID,
		Status:      input.Status,
		Limit:       limit,
		Cursor:      input.Cursor,
	})
	if err != nil {
		s.logger.Error("failed to list api keys", "error", err, "workspace_id", input.WorkspaceID)
		return nil, err
	}

	results := make([]APIKeyResult, 0, len(keys))
	for _, k := range keys {
		results = append(results, apiKeyToResult(k))
	}

	return &ListAPIKeysResult{APIKeys: results, Cursor: cursor}, nil
}

func (s *Service) CreateAPIKey(ctx context.Context, input CreateAPIKeyInput) (*CreateAPIKeyResult, error) {
	log := s.logger.With("usecase", "CreateAPIKey", "workspace_id", input.WorkspaceID, "actor_user_id", input.ActorUserID)

	if input.WorkspaceID == "" || input.ActorUserID == "" {
		return nil, domain.ErrPayloadInvalid
	}

	if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.ActorUserID, "api_key.manage"); err != nil {
		return nil, err
	}

	name := domain.NormalizeName(input.Name)
	if name == "" || len(name) > 120 {
		return nil, domain.ErrAPIKeyConfigInvalid
	}

	if err := domain.ValidateScopes(input.Scopes); err != nil {
		log.Warn("invalid scopes for create", "scopes", input.Scopes)
		return nil, err
	}

	now := time.Now().UTC()

	if input.ExpiresAt != nil && !input.ExpiresAt.IsZero() && input.ExpiresAt.Before(now) {
		return nil, domain.ErrAPIKeyConfigInvalid
	}

	id, err := s.idGen()
	if err != nil {
		log.Error("failed to generate id", "error", err)
		return nil, err
	}

	plaintext, keyPrefix, err := s.secretGen.Generate()
	if err != nil {
		log.Error("failed to generate secret", "error", err)
		return nil, err
	}

	secretHash, err := s.secretHasher.Hash(plaintext)
	if err != nil {
		log.Error("failed to hash secret", "error", err)
		return nil, err
	}

	key := domain.APIKey{
		ID:          id,
		WorkspaceID: input.WorkspaceID,
		Name:        name,
		KeyPrefix:   keyPrefix,
		SecretHash:  secretHash,
		Scopes:      input.Scopes,
		Status:      domain.APIKeyStatusActive,
		CreatedAt:   now,
		UpdatedAt:   now,
		ExpiresAt:   input.ExpiresAt,
	}

	if err := s.apiKeyRepo.Create(ctx, key); err != nil {
		log.Error("failed to persist api key", "error", err)
		return nil, err
	}

	log.Info("api key created", "api_key_id", id)

	result := apiKeyToResult(key)
	return &CreateAPIKeyResult{APIKeyResult: result, Secret: plaintext}, nil
}

func (s *Service) UpdateAPIKey(ctx context.Context, input UpdateAPIKeyInput) (*UpdateAPIKeyResult, error) {
	log := s.logger.With("usecase", "UpdateAPIKey", "workspace_id", input.WorkspaceID, "api_key_id", input.APIKeyID, "actor_user_id", input.ActorUserID)

	if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.ActorUserID, "api_key.manage"); err != nil {
		return nil, err
	}

	existing, err := s.apiKeyRepo.FindByID(ctx, input.WorkspaceID, input.APIKeyID)
	if err != nil {
		return nil, err
	}

	if existing.Status != domain.APIKeyStatusActive {
		log.Warn("attempted to update non-active key", "status", existing.Status)
		return nil, domain.ErrAPIKeyRotateConflict
	}

	now := time.Now().UTC()

	if input.Name != nil {
		name := domain.NormalizeName(*input.Name)
		if name == "" || len(name) > 120 {
			return nil, domain.ErrAPIKeyConfigInvalid
		}
		existing.Name = name
	}

	if input.Scopes != nil {
		if err := domain.ValidateScopes(input.Scopes); err != nil {
			log.Warn("invalid scopes for update", "scopes", input.Scopes)
			return nil, err
		}
		existing.Scopes = input.Scopes
	}

	if input.ExpiresAt != nil {
		if !input.ExpiresAt.IsZero() && input.ExpiresAt.Before(now) {
			return nil, domain.ErrAPIKeyConfigInvalid
		}
		existing.ExpiresAt = input.ExpiresAt
	}

	var newSecret string

	if input.Rotate {
		plaintext, keyPrefix, err := s.secretGen.Generate()
		if err != nil {
			log.Error("failed to generate secret for rotation", "error", err)
			return nil, err
		}

		secretHash, err := s.secretHasher.Hash(plaintext)
		if err != nil {
			log.Error("failed to hash secret for rotation", "error", err)
			return nil, err
		}

		existing.KeyPrefix = keyPrefix
		existing.SecretHash = secretHash
		newSecret = plaintext
	}

	existing.UpdatedAt = now

	if err := s.apiKeyRepo.Update(ctx, *existing); err != nil {
		log.Error("failed to update api key", "error", err)
		return nil, err
	}

	log.Info("api key updated", "rotated", input.Rotate)

	result := apiKeyToResult(*existing)
	return &UpdateAPIKeyResult{APIKeyResult: result, Secret: newSecret}, nil
}

func (s *Service) RevokeAPIKey(ctx context.Context, input RevokeAPIKeyInput) (*APIKeyResult, error) {
	log := s.logger.With("usecase", "RevokeAPIKey", "workspace_id", input.WorkspaceID, "api_key_id", input.APIKeyID, "actor_user_id", input.ActorUserID)

	if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.ActorUserID, "api_key.manage"); err != nil {
		return nil, err
	}

	existing, err := s.apiKeyRepo.FindByID(ctx, input.WorkspaceID, input.APIKeyID)
	if err != nil {
		return nil, err
	}

	if existing.Status == domain.APIKeyStatusRevoked {
		log.Info("api key already revoked, returning success")
		result := apiKeyToResult(*existing)
		return &result, nil
	}

	now := time.Now().UTC()
	existing.Status = domain.APIKeyStatusRevoked
	existing.RevokedAt = &now
	existing.UpdatedAt = now

	if err := s.apiKeyRepo.Update(ctx, *existing); err != nil {
		log.Error("failed to revoke api key", "error", err)
		return nil, err
	}

	log.Info("api key revoked")

	result := apiKeyToResult(*existing)
	return &result, nil
}

func (s *Service) AuthenticateAPIKey(ctx context.Context, input AuthenticateAPIKeyInput) (*AuthenticatedAPIKey, error) {
	log := s.logger.With("usecase", "AuthenticateAPIKey")

	token := input.BearerToken
	if token == "" {
		return nil, domain.ErrAPIKeyInvalid
	}

	prefix := domain.DerivePrefix(token)

	key, err := s.apiKeyRepo.FindByPrefix(ctx, prefix)
	if err != nil {
		if errors.Is(err, domain.ErrAPIKeyNotFound) {
			return nil, domain.ErrAPIKeyInvalid
		}
		log.Error("failed to find api key by prefix", "error", err)
		return nil, domain.ErrAPIKeyInvalid
	}

	if !s.secretHasher.Verify(key.SecretHash, token) {
		return nil, domain.ErrAPIKeyInvalid
	}

	if !domain.IsActive(*key, time.Now().UTC()) {
		return nil, domain.ErrAPIKeyInvalid
	}

	if err := s.apiKeyRepo.TouchLastUsed(ctx, key.WorkspaceID, key.ID, time.Now().UTC()); err != nil {
		log.Error("failed to touch last_used_at", "error", err, "api_key_id", key.ID)
	}

	return &AuthenticatedAPIKey{
		WorkspaceID: key.WorkspaceID,
		APIKeyID:    key.ID,
		Scopes:      key.Scopes,
		KeyPrefix:   key.KeyPrefix,
	}, nil
}

func (s *Service) RequireAPIKeyScope(key *AuthenticatedAPIKey, scope string) error {
	if !domain.HasScope(key.Scopes, scope) {
		return domain.ErrAPIKeyScopeDenied
	}
	return nil
}

func apiKeyToResult(k domain.APIKey) APIKeyResult {
	return APIKeyResult{
		ID:          k.ID,
		WorkspaceID: k.WorkspaceID,
		Name:        k.Name,
		KeyPrefix:   k.KeyPrefix,
		Scopes:      k.Scopes,
		Status:      string(k.Status),
		CreatedAt:   k.CreatedAt,
		UpdatedAt:   k.UpdatedAt,
		LastUsedAt:  k.LastUsedAt,
		ExpiresAt:   k.ExpiresAt,
		RevokedAt:   k.RevokedAt,
	}
}
