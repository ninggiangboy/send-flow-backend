package infrastructure

import (
	"strings"
	"testing"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/access/domain"
)

func TestCryptoSecretGenerator_GeneratesNonEmpty(t *testing.T) {
	gen := NewCryptoSecretGenerator()
	secret, prefix, err := gen.Generate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if secret == "" {
		t.Fatal("expected non-empty secret")
	}
	if !strings.HasPrefix(secret, domain.SecretStablePrefix) {
		t.Errorf("expected secret to start with %q, got %q", domain.SecretStablePrefix, secret)
	}
	if prefix == "" {
		t.Fatal("expected non-empty prefix")
	}
}

func TestCryptoSecretGenerator_Unique(t *testing.T) {
	gen := NewCryptoSecretGenerator()
	seen := make(map[string]bool)
	for range 10 {
		s, _, err := gen.Generate()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if seen[s] {
			t.Fatal("generated duplicate secret")
		}
		seen[s] = true
	}
}

func TestBcryptSecretHasher_ValidatesCorrectSecret(t *testing.T) {
	hasher := NewBcryptSecretHasher()
	secret := "sk_test_secret_value_12345"
	hash, err := hasher.Hash(secret)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasher.Verify(hash, secret) {
		t.Error("expected verification to succeed")
	}
}

func TestBcryptSecretHasher_RejectsWrongSecret(t *testing.T) {
	hasher := NewBcryptSecretHasher()
	secret := "sk_test_secret_value_12345"
	wrong := "sk_test_wrong_secret"
	hash, err := hasher.Hash(secret)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasher.Verify(hash, wrong) {
		t.Error("expected verification to fail for wrong secret")
	}
}

func TestBcryptSecretHasher_EmptySecret(t *testing.T) {
	hasher := NewBcryptSecretHasher()
	_, err := hasher.Hash("")
	if err == nil {
		t.Error("expected error for empty secret")
	}
}
