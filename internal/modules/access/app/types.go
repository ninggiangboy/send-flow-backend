package app

import (
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/access/app/apikey"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/access/domain"
)

// Input type aliases — backward compatible with API-layer callers.
type (
	ListAPIKeysInput    = apikey.ListQuery
	CreateAPIKeyInput   = apikey.CreateInput
	UpdateAPIKeyInput   = apikey.UpdateInput
	RevokeAPIKeyInput   = apikey.RevokeInput
	AuthenticatedAPIKey = apikey.AuthenticateResult
)

// AuthenticateAPIKeyInput is a concrete struct (not an alias) because it is
// constructed by the API layer with a single BearerToken field that does not
// match the grouped command struct shape of other input types.
type AuthenticateAPIKeyInput struct {
	BearerToken string
}

// APIKeyResult is a concrete DTO with JSON tags used by the API in responses.
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

type ListAPIKeysResult struct {
	APIKeys []APIKeyResult
	Cursor  string
}

type CreateAPIKeyResult struct {
	APIKeyResult
	Secret string `json:"secret"`
}

type UpdateAPIKeyResult struct {
	APIKeyResult
	Secret string `json:"secret,omitempty"`
}
