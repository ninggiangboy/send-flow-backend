package domain

import (
	"slices"
	"strings"
	"time"
)

type APIKeyStatus string

const (
	APIKeyStatusActive  APIKeyStatus = "active"
	APIKeyStatusRevoked APIKeyStatus = "revoked"
)

const (
	ScopeTransactionalSend = "transactional.send"
	ScopeTransactionalRead = "transactional.read"
)

var supportedScopes = []string{
	ScopeTransactionalSend,
	ScopeTransactionalRead,
}

type APIKey struct {
	ID          string
	WorkspaceID string
	Name        string
	KeyPrefix   string
	SecretHash  string
	Scopes      []string
	Status      APIKeyStatus
	CreatedAt   time.Time
	UpdatedAt   time.Time
	LastUsedAt  *time.Time
	ExpiresAt   *time.Time
	RevokedAt   *time.Time
}

func SupportedScopes() []string {
	return slices.Clone(supportedScopes)
}

func ValidateScopes(scopes []string) error {
	if len(scopes) == 0 {
		return ErrAPIKeyScopeInvalid
	}
	for _, s := range scopes {
		if !slices.Contains(supportedScopes, s) {
			return ErrAPIKeyScopeInvalid
		}
	}
	return nil
}

func HasScope(scopes []string, target string) bool {
	return slices.Contains(scopes, target)
}

func IsActive(key APIKey, now time.Time) bool {
	if key.Status != APIKeyStatusActive {
		return false
	}
	if key.ExpiresAt != nil && now.After(*key.ExpiresAt) {
		return false
	}
	if key.RevokedAt != nil {
		return false
	}
	return true
}

func NormalizeName(name string) string {
	return strings.TrimSpace(name)
}
