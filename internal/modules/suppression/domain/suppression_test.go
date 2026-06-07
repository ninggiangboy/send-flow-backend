package domain

import "testing"

func TestValidSuppressionScope(t *testing.T) {
	if !ValidSuppressionScope("workspace") {
		t.Error("expected workspace to be valid")
	}
	if !ValidSuppressionScope("list") {
		t.Error("expected list to be valid")
	}
	if !ValidSuppressionScope("global") {
		t.Error("expected global to be valid")
	}
	if ValidSuppressionScope("invalid") {
		t.Error("expected invalid scope to be invalid")
	}
}

func TestValidSuppressionReason(t *testing.T) {
	if !ValidSuppressionReason("manual_block") {
		t.Error("expected manual_block to be valid")
	}
	if !ValidSuppressionReason("bounce") {
		t.Error("expected bounce to be valid")
	}
	if !ValidSuppressionReason("complaint") {
		t.Error("expected complaint to be valid")
	}
	if !ValidSuppressionReason("unsubscribe") {
		t.Error("expected unsubscribe to be valid")
	}
	if ValidSuppressionReason("invalid") {
		t.Error("expected invalid reason to be invalid")
	}
}

func TestNormalizeEmail(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Alice@Example.com", "alice@example.com"},
		{"  BOB@Example.ORG  ", "bob@example.org"},
		{"", ""},
	}

	for _, tt := range tests {
		got := NormalizeEmail(tt.input)
		if got != tt.expected {
			t.Errorf("NormalizeEmail(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestSuppressionEntryDefaults(t *testing.T) {
	e := SuppressionEntry{
		ID:     "sup_1",
		Email:  "test@example.com",
		Scope:  SuppressionScopeWorkspace,
		Reason: SuppressionReasonManualBlock,
		Status: SuppressionStatusActive,
	}

	if e.ID != "sup_1" {
		t.Errorf("expected ID sup_1, got %s", e.ID)
	}
	if string(e.Scope) != "workspace" {
		t.Errorf("expected scope workspace, got %s", e.Scope)
	}
	if string(e.Reason) != "manual_block" {
		t.Errorf("expected reason manual_block, got %s", e.Reason)
	}
	if string(e.Status) != "active" {
		t.Errorf("expected status active, got %s", e.Status)
	}
}
