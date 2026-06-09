package domain

import (
	"testing"
)

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