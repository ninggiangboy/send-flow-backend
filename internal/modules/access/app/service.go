package app

import (
	"context"
	"log/slog"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/access/app/apikey"
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
	listAPIKeysH        *apikey.ListHandler
	createAPIKeyH       *apikey.CreateHandler
	updateAPIKeyH       *apikey.UpdateHandler
	revokeAPIKeyH       *apikey.RevokeHandler
	authenticateAPIKeyH *apikey.AuthenticateHandler
}

func NewService(opts Options) *Service {
	return &Service{
		listAPIKeysH: apikey.NewListHandler(struct {
			APIKeyRepo    ports.APIKeyRepository
			AccessChecker ports.WorkspaceAccessChecker
			Logger        *slog.Logger
		}{
			APIKeyRepo:    opts.APIKeyRepo,
			AccessChecker: opts.AccessChecker,
			Logger:        opts.Logger,
		}),
		createAPIKeyH: apikey.NewCreateHandler(struct {
			APIKeyRepo    ports.APIKeyRepository
			AccessChecker ports.WorkspaceAccessChecker
			IDGen         ports.IDGenerator
			SecretGen     ports.SecretGenerator
			SecretHasher  ports.SecretHasher
			Logger        *slog.Logger
		}{
			APIKeyRepo:    opts.APIKeyRepo,
			AccessChecker: opts.AccessChecker,
			IDGen:         opts.IDGen,
			SecretGen:     opts.SecretGen,
			SecretHasher:  opts.SecretHasher,
			Logger:        opts.Logger,
		}),
		updateAPIKeyH: apikey.NewUpdateHandler(struct {
			APIKeyRepo    ports.APIKeyRepository
			AccessChecker ports.WorkspaceAccessChecker
			SecretGen     ports.SecretGenerator
			SecretHasher  ports.SecretHasher
			Logger        *slog.Logger
		}{
			APIKeyRepo:    opts.APIKeyRepo,
			AccessChecker: opts.AccessChecker,
			SecretGen:     opts.SecretGen,
			SecretHasher:  opts.SecretHasher,
			Logger:        opts.Logger,
		}),
		revokeAPIKeyH: apikey.NewRevokeHandler(struct {
			APIKeyRepo    ports.APIKeyRepository
			AccessChecker ports.WorkspaceAccessChecker
			Logger        *slog.Logger
		}{
			APIKeyRepo:    opts.APIKeyRepo,
			AccessChecker: opts.AccessChecker,
			Logger:        opts.Logger,
		}),
		authenticateAPIKeyH: apikey.NewAuthenticateHandler(struct {
			APIKeyRepo   ports.APIKeyRepository
			SecretHasher ports.SecretHasher
			Logger       *slog.Logger
		}{
			APIKeyRepo:   opts.APIKeyRepo,
			SecretHasher: opts.SecretHasher,
			Logger:       opts.Logger,
		}),
	}
}

func (s *Service) ListAPIKeys(ctx context.Context, input ListAPIKeysInput) (*ListAPIKeysResult, error) {
	keys, cursor, err := s.listAPIKeysH.Execute(ctx, input)
	if err != nil {
		return nil, err
	}

	results := make([]APIKeyResult, 0, len(keys))
	for _, k := range keys {
		results = append(results, apiKeyToResult(k))
	}

	return &ListAPIKeysResult{APIKeys: results, Cursor: cursor}, nil
}

func (s *Service) CreateAPIKey(ctx context.Context, input CreateAPIKeyInput) (*CreateAPIKeyResult, error) {
	result, err := s.createAPIKeyH.Execute(ctx, input)
	if err != nil {
		return nil, err
	}

	return &CreateAPIKeyResult{
		APIKeyResult: apiKeyToResult(result.APIKey),
		Secret:       result.Secret,
	}, nil
}

func (s *Service) UpdateAPIKey(ctx context.Context, input UpdateAPIKeyInput) (*UpdateAPIKeyResult, error) {
	result, err := s.updateAPIKeyH.Execute(ctx, input)
	if err != nil {
		return nil, err
	}

	return &UpdateAPIKeyResult{
		APIKeyResult: apiKeyToResult(result.APIKey),
		Secret:       result.Secret,
	}, nil
}

func (s *Service) RevokeAPIKey(ctx context.Context, input RevokeAPIKeyInput) (*APIKeyResult, error) {
	key, err := s.revokeAPIKeyH.Execute(ctx, input)
	if err != nil {
		return nil, err
	}

	result := apiKeyToResult(*key)
	return &result, nil
}

func (s *Service) AuthenticateAPIKey(ctx context.Context, input AuthenticateAPIKeyInput) (*AuthenticatedAPIKey, error) {
	return s.authenticateAPIKeyH.Execute(ctx, apikey.AuthenticateInput(input))
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
		QuotaLimits: k.QuotaLimits,
	}
}
