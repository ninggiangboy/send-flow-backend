package domain

import "testing"

func TestNormalizeEmail(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Alice@Example.com", "alice@example.com"},
		{"  BOB@Example.ORG  ", "bob@example.org"},
		{"", ""},
		{"UPPERCASE@DOMAIN.COM", "uppercase@domain.com"},
	}

	for _, tt := range tests {
		got := NormalizeEmail(tt.input)
		if got != tt.expected {
			t.Errorf("NormalizeEmail(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestValidContactStatus(t *testing.T) {
	if !ValidContactStatus("active") {
		t.Error("expected active to be valid")
	}
	if !ValidContactStatus("archived") {
		t.Error("expected archived to be valid")
	}
	if ValidContactStatus("invalid") {
		t.Error("expected invalid status to be invalid")
	}
	if ValidContactStatus("") {
		t.Error("expected empty to be invalid")
	}
}

func TestValidSegmentStatus(t *testing.T) {
	if !ValidSegmentStatus("ready") {
		t.Error("expected ready to be valid")
	}
	if !ValidSegmentStatus("processing") {
		t.Error("expected processing to be valid")
	}
	if !ValidSegmentStatus("disabled") {
		t.Error("expected disabled to be valid")
	}
	if ValidSegmentStatus("invalid") {
		t.Error("expected invalid to be invalid")
	}
}

func TestValidDedupeMode(t *testing.T) {
	if !ValidDedupeMode("by_email") {
		t.Error("expected by_email to be valid")
	}
	if ValidDedupeMode("invalid") {
		t.Error("expected invalid dedupe mode to be invalid")
	}
}

func TestValidExportFormat(t *testing.T) {
	if !ValidExportFormat("csv") {
		t.Error("expected csv to be valid")
	}
	if ValidExportFormat("json") {
		t.Error("expected json format to be invalid")
	}
}

func TestValidListMembershipMode(t *testing.T) {
	if !ValidListMembershipMode("merge") {
		t.Error("expected merge to be valid")
	}
	if !ValidListMembershipMode("replace") {
		t.Error("expected replace to be valid")
	}
	if ValidListMembershipMode("invalid") {
		t.Error("expected invalid mode to be invalid")
	}
}

func TestValidJobStatus(t *testing.T) {
	if !ValidJobStatus("queued") {
		t.Error("expected queued to be valid")
	}
	if !ValidJobStatus("running") {
		t.Error("expected running to be valid")
	}
	if !ValidJobStatus("completed") {
		t.Error("expected completed to be valid")
	}
	if !ValidJobStatus("failed") {
		t.Error("expected failed to be valid")
	}
	if ValidJobStatus("invalid") {
		t.Error("expected invalid job status to be invalid")
	}
}

func TestContactDefaults(t *testing.T) {
	c := Contact{
		ID:     "ct_1",
		Email:  "test@example.com",
		Status: ContactStatusActive,
	}

	if c.ID != "ct_1" {
		t.Errorf("expected ID ct_1, got %s", c.ID)
	}
	if string(c.Status) != "active" {
		t.Errorf("expected status active, got %s", c.Status)
	}
}

func TestSegmentDefinitionValidation(t *testing.T) {
	s := Segment{
		ID:             "seg_1",
		Name:           "test segment",
		DefinitionJSON: map[string]any{"rules": []any{map[string]any{"field": "email", "op": "eq", "value": "test@example.com"}}},
		Status:         SegmentStatusReady,
	}

	if s.Name != "test segment" {
		t.Errorf("expected name 'test segment', got %s", s.Name)
	}
	if s.Status != SegmentStatusReady {
		t.Errorf("expected status ready, got %s", s.Status)
	}
	rules, ok := s.DefinitionJSON["rules"].([]any)
	if !ok || len(rules) != 1 {
		t.Error("expected definition to have 1 rule")
	}
}
