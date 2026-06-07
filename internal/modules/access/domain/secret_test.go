package domain

import (
	"strings"
	"testing"
)

func TestSecretGenerator_GeneratesNonEmpty(t *testing.T) {
	gen := NewSecretGenerator()
	secret, prefix, err := gen.Generate()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if secret == "" {
		t.Fatal("expected non-empty secret")
	}
	if !strings.HasPrefix(secret, SecretStablePrefix) {
		t.Errorf("expected secret to start with %q, got %q", SecretStablePrefix, secret)
	}
	if prefix == "" {
		t.Fatal("expected non-empty prefix")
	}
}

func TestSecretGenerator_Unique(t *testing.T) {
	gen := NewSecretGenerator()
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

func TestDerivePrefix_Deterministic(t *testing.T) {
	secret := "sk_test_abc123def456ghi789"
	p1 := DerivePrefix(secret)
	p2 := DerivePrefix(secret)
	if p1 != p2 {
		t.Errorf("prefix should be deterministic, got %q and %q", p1, p2)
	}
	if len(p1) > PrefixVisibleChars {
		t.Errorf("prefix should be at most %d chars, got %d", PrefixVisibleChars, len(p1))
	}
}

func TestSecretHasher_ValidatesCorrectSecret(t *testing.T) {
	hasher := NewSecretHasher()
	secret := "sk_test_secret_value_12345"
	hash, err := hasher.Hash(secret)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasher.Verify(hash, secret) {
		t.Error("expected verification to succeed")
	}
}

func TestSecretHasher_RejectsWrongSecret(t *testing.T) {
	hasher := NewSecretHasher()
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

func TestSecretHasher_EmptySecret(t *testing.T) {
	hasher := NewSecretHasher()
	_, err := hasher.Hash("")
	if err == nil {
		t.Error("expected error for empty secret")
	}
}

func TestExtractBearerToken_Valid(t *testing.T) {
	token, ok := ExtractBearerToken("Bearer sk_test_token")
	if !ok {
		t.Fatal("expected successful extraction")
	}
	if token != "sk_test_token" {
		t.Errorf("expected %q, got %q", "sk_test_token", token)
	}
}

func TestExtractBearerToken_Missing(t *testing.T) {
	_, ok := ExtractBearerToken("")
	if ok {
		t.Fatal("expected failure for empty header")
	}
}

func TestExtractBearerToken_NoBearer(t *testing.T) {
	_, ok := ExtractBearerToken("Basic dGVzdDp0ZXN0")
	if ok {
		t.Fatal("expected failure for non-bearer auth")
	}
}

func TestExtractBearerToken_EmptyToken(t *testing.T) {
	_, ok := ExtractBearerToken("Bearer ")
	if ok {
		t.Fatal("expected failure for empty token")
	}
}
