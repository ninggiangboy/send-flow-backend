package secrets

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestEnvProvider_Get(t *testing.T) {
	t.Setenv("APP_JWT_SIGNING_KEY", "secret")

	got, err := NewEnvProvider("APP_").Get(context.Background(), "jwt-signing-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "secret" {
		t.Fatalf("unexpected secret: %s", got)
	}
}

func TestStaticProvider_WhenMissing_ReturnsNotFound(t *testing.T) {
	_, err := StaticProvider{}.Get(context.Background(), "missing")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestFileProvider_Get(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secrets.json")
	if err := os.WriteFile(path, []byte(`{"jwt-signing-key":"secret"}`), 0o600); err != nil {
		t.Fatalf("write secret file: %v", err)
	}

	provider, err := NewFileProvider(path)
	if err != nil {
		t.Fatalf("unexpected provider error: %v", err)
	}
	got, err := provider.Get(context.Background(), "jwt-signing-key")
	if err != nil {
		t.Fatalf("unexpected get error: %v", err)
	}
	if got != "secret" {
		t.Fatalf("unexpected secret: %s", got)
	}
}
