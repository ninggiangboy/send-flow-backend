package app

import (
	"context"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/access/app/authenticateapikey"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/access/app/createapikey"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/access/app/listapikeys"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/access/app/revokeapikey"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/access/app/updateapikey"
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
	listAPIKeysH        *listapikeys.Handler
	createAPIKeyH       *createapikey.Handler
	updateAPIKeyH       *updateapikey.Handler
	revokeAPIKeyH       *revokeapikey.Handler
	authenticateAPIKeyH *authenticateapikey.Handler
}

func NewService(opts Options) *Service {
	return &Service{
		listAPIKeysH: listapikeys.New(listapikeys.Options{
			APIKeyRepo:    opts.APIKeyRepo,
			AccessChecker: opts.AccessChecker,
			Logger:        opts.Logger,
		}),
		createAPIKeyH: createapikey.New(createapikey.Options{
			APIKeyRepo:    opts.APIKeyRepo,
			AccessChecker: opts.AccessChecker,
			IDGen:         opts.IDGen,
			SecretGen:     opts.SecretGen,
			SecretHasher:  opts.SecretHasher,
			Logger:        opts.Logger,
		}),
		updateAPIKeyH: updateapikey.New(updateapikey.Options{
			APIKeyRepo:    opts.APIKeyRepo,
			AccessChecker: opts.AccessChecker,
			SecretGen:     opts.SecretGen,
			SecretHasher:  opts.SecretHasher,
			Logger:        opts.Logger,
		}),
		revokeAPIKeyH: revokeapikey.New(revokeapikey.Options{
			APIKeyRepo:    opts.APIKeyRepo,
			AccessChecker: opts.AccessChecker,
			Logger:        opts.Logger,
		}),
		authenticateAPIKeyH: authenticateapikey.New(authenticateapikey.Options{
			APIKeyRepo:   opts.APIKeyRepo,
			SecretHasher: opts.SecretHasher,
			Logger:       opts.Logger,
		}),
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
	ID          string                   `json:"id"`
	WorkspaceID string                   `json:"workspace_id"`
	Name        string                   `json:"name"`
	KeyPrefix   string                   `json:"key_prefix"`
	Scopes      []string                 `json:"scopes"`
	Status      string                   `json:"status"`
	CreatedAt   time.Time                `json:"created_at"`
	UpdatedAt   time.Time                `json:"updated_at"`
	LastUsedAt  *time.Time               `json:"last_used_at"`
	ExpiresAt   *time.Time               `json:"expires_at"`
	RevokedAt   *time.Time               `json:"revoked_at"`
	QuotaLimits *domain.EmailQuotaLimits `json:"email_quota_limits,omitempty"`
}

type CreateAPIKeyInput struct {
	WorkspaceID string
	ActorUserID string
	Name        string
	Scopes      []string
	ExpiresAt   *time.Time
	QuotaLimits *domain.EmailQuotaLimits
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
	QuotaLimits *domain.EmailQuotaLimits
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
	keys, cursor, err := s.listAPIKeysH.Execute(ctx, listapikeys.Command{
		WorkspaceID: input.WorkspaceID,
		ActorUserID: input.ActorUserID,
		Status:      input.Status,
		Limit:       input.Limit,
		Cursor:      input.Cursor,
	})
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
	result, err := s.createAPIKeyH.Execute(ctx, createapikey.Command{
		WorkspaceID: input.WorkspaceID,
		ActorUserID: input.ActorUserID,
		Name:        input.Name,
		Scopes:      input.Scopes,
		ExpiresAt:   input.ExpiresAt,
		QuotaLimits: input.QuotaLimits,
	})
	if err != nil {
		return nil, err
	}

	return &CreateAPIKeyResult{
		APIKeyResult: apiKeyToResult(result.APIKey),
		Secret:       result.Secret,
	}, nil
}

func (s *Service) UpdateAPIKey(ctx context.Context, input UpdateAPIKeyInput) (*UpdateAPIKeyResult, error) {
	result, err := s.updateAPIKeyH.Execute(ctx, updateapikey.Command{
		WorkspaceID: input.WorkspaceID,
		ActorUserID: input.ActorUserID,
		APIKeyID:    input.APIKeyID,
		Name:        input.Name,
		Scopes:      input.Scopes,
		ExpiresAt:   input.ExpiresAt,
		Rotate:      input.Rotate,
		QuotaLimits: input.QuotaLimits,
	})
	if err != nil {
		return nil, err
	}

	return &UpdateAPIKeyResult{
		APIKeyResult: apiKeyToResult(result.APIKey),
		Secret:       result.Secret,
	}, nil
}

func (s *Service) RevokeAPIKey(ctx context.Context, input RevokeAPIKeyInput) (*APIKeyResult, error) {
	key, err := s.revokeAPIKeyH.Execute(ctx, revokeapikey.Command{
		WorkspaceID: input.WorkspaceID,
		ActorUserID: input.ActorUserID,
		APIKeyID:    input.APIKeyID,
	})
	if err != nil {
		return nil, err
	}

	result := apiKeyToResult(*key)
	return &result, nil
}

func (s *Service) AuthenticateAPIKey(ctx context.Context, input AuthenticateAPIKeyInput) (*AuthenticatedAPIKey, error) {
	result, err := s.authenticateAPIKeyH.Execute(ctx, input.BearerToken)
	if err != nil {
		return nil, err
	}

	return &AuthenticatedAPIKey{
		WorkspaceID: result.WorkspaceID,
		APIKeyID:    result.APIKeyID,
		Scopes:      result.Scopes,
		KeyPrefix:   result.KeyPrefix,
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
		QuotaLimits: k.QuotaLimits,
	}
}
