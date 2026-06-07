package domain

import (
	"testing"
	"time"
)

func TestSupportedScopes(t *testing.T) {
	scopes := SupportedScopes()
	if len(scopes) == 0 {
		t.Fatal("expected at least one supported scope")
	}
	expected := []string{ScopeTransactionalSend, ScopeTransactionalRead}
	for _, exp := range expected {
		found := false
		for _, s := range scopes {
			if s == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected scope %q to be in supported list", exp)
		}
	}
}

func TestValidateScopes_Valid(t *testing.T) {
	if err := ValidateScopes([]string{ScopeTransactionalSend}); err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if err := ValidateScopes([]string{ScopeTransactionalSend, ScopeTransactionalRead}); err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestValidateScopes_Empty(t *testing.T) {
	if err := ValidateScopes([]string{}); err != ErrAPIKeyScopeInvalid {
		t.Errorf("expected ErrAPIKeyScopeInvalid for empty scopes, got %v", err)
	}
}

func TestValidateScopes_Unknown(t *testing.T) {
	if err := ValidateScopes([]string{"unknown.scope"}); err != ErrAPIKeyScopeInvalid {
		t.Errorf("expected ErrAPIKeyScopeInvalid for unknown scope, got %v", err)
	}
}

func TestHasScope(t *testing.T) {
	if !HasScope([]string{ScopeTransactionalSend}, ScopeTransactionalSend) {
		t.Error("expected HasScope to return true")
	}
	if HasScope([]string{ScopeTransactionalSend}, ScopeTransactionalRead) {
		t.Error("expected HasScope to return false for missing scope")
	}
	if HasScope([]string{}, ScopeTransactionalSend) {
		t.Error("expected HasScope to return false for empty scopes")
	}
}

func TestIsActive_ActiveKey(t *testing.T) {
	now := time.Now().UTC()
	key := APIKey{
		Status:    APIKeyStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if !IsActive(key, now) {
		t.Error("expected active key to be active")
	}

	if !IsActive(key, now.Add(time.Hour)) {
		t.Error("expected active key without expiration to be active in the future")
	}
}

func TestIsActive_RevokedKey(t *testing.T) {
	now := time.Now().UTC()
	revokedAt := now
	key := APIKey{
		Status:    APIKeyStatusRevoked,
		RevokedAt: &revokedAt,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if IsActive(key, now) {
		t.Error("expected revoked key to not be active")
	}
}

func TestIsActive_ExpiredKey(t *testing.T) {
	now := time.Now().UTC()
	expiresAt := now.Add(-time.Hour)
	key := APIKey{
		Status:    APIKeyStatusActive,
		ExpiresAt: &expiresAt,
		CreatedAt: now.Add(-2 * time.Hour),
		UpdatedAt: now.Add(-2 * time.Hour),
	}
	if IsActive(key, now) {
		t.Error("expected expired key to not be active")
	}
}

func TestIsActive_NotExpired(t *testing.T) {
	now := time.Now().UTC()
	expiresAt := now.Add(time.Hour)
	key := APIKey{
		Status:    APIKeyStatusActive,
		ExpiresAt: &expiresAt,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if !IsActive(key, now) {
		t.Error("expected non-expired key to be active")
	}
}

func TestNormalizeName(t *testing.T) {
	if got := NormalizeName("  My Key  "); got != "My Key" {
		t.Errorf("expected %q, got %q", "My Key", got)
	}
	if got := NormalizeName(""); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}
