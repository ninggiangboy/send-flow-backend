package domain

import (
	"encoding/json"
	"testing"
)

func TestIsValidTargetType(t *testing.T) {
	if !IsValidTargetType(ReplayTargetDeadLetter) {
		t.Fatal("expected dead_letter_record to be valid")
	}
	if !IsValidTargetType(ReplayTargetOutbox) {
		t.Fatal("expected outbox_event to be valid")
	}
	if IsValidTargetType("invalid_target") {
		t.Fatal("expected invalid target to be rejected")
	}
}

func TestIsValidReplayStatus(t *testing.T) {
	cases := []struct {
		status ReplayJobStatus
		valid  bool
	}{
		{ReplayJobQueued, true},
		{ReplayJobRunning, true},
		{ReplayJobCompleted, true},
		{ReplayJobFailed, true},
		{"invalid", false},
		{"cancelled", false},
	}
	for _, tc := range cases {
		if got := IsValidReplayStatus(tc.status); got != tc.valid {
			t.Errorf("IsValidReplayStatus(%q) = %v, want %v", tc.status, got, tc.valid)
		}
	}
}

func TestSanitizeErrorMessage(t *testing.T) {
	short := "short error"
	if got := SanitizeErrorMessage(short); got != short {
		t.Fatal("short error should not be truncated")
	}

	long := make([]byte, 3000)
	for i := range long {
		long[i] = 'a'
	}
	longStr := string(long)
	got := SanitizeErrorMessage(longStr)
	if len(got) != 2000 {
		t.Fatalf("expected 2000 chars, got %d", len(got))
	}
}

func TestRedactSensitiveFields(t *testing.T) {
	input := json.RawMessage(`{
		"event_type": "delivery.message.delivered.v1",
		"authorization": "Bearer secret123",
		"nested": {
			"token": "sensitive",
			"safe": "value"
		},
		"items": [
			{"password": "secret"},
			{"api_key": "key123"}
		]
	}`)

	redacted := RedactSensitiveFields(input)

	var result map[string]any
	if err := json.Unmarshal(redacted, &result); err != nil {
		t.Fatalf("failed to unmarshal redacted: %v", err)
	}

	if result["authorization"] != "[REDACTED]" {
		t.Fatal("authorization should be redacted")
	}

	nested, ok := result["nested"].(map[string]any)
	if !ok {
		t.Fatal("nested should be a map")
	}
	if nested["token"] != "[REDACTED]" {
		t.Fatal("nested token should be redacted")
	}
	if nested["safe"] != "value" {
		t.Fatal("safe field should not be redacted")
	}

	items, ok := result["items"].([]any)
	if !ok {
		t.Fatal("items should be an array")
	}
	if items[0].(map[string]any)["password"] != "[REDACTED]" {
		t.Fatal("password in array should be redacted")
	}
	if items[1].(map[string]any)["api_key"] != "[REDACTED]" {
		t.Fatal("api_key in array should be redacted")
	}
}

func TestRedactSensitiveFieldsNonObject(t *testing.T) {
	input := json.RawMessage(`"just a string"`)
	redacted := RedactSensitiveFields(input)
	var result string
	if err := json.Unmarshal(redacted, &result); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if result != "just a string" {
		t.Fatal("non-object payload should pass through unchanged")
	}
}

func TestSanitizePayloadPreview(t *testing.T) {
	data := json.RawMessage(`{"key": "value"}`)
	preview := SanitizePayloadPreview(data, 100)
	if len(preview) != len(data) {
		t.Fatal("preview should not truncate when under limit")
	}

	longData := json.RawMessage(make([]byte, 200))
	preview = SanitizePayloadPreview(longData, 50)
	if string(preview) != "{}" {
		t.Fatalf("expected {} for truncated invalid JSON, got %s", string(preview))
	}

	validLongData := json.RawMessage(`{"key": "a very long value that needs truncation for testing purposes"}`)
	preview = SanitizePayloadPreview(validLongData, 30)
	if len(preview) > 30 {
		t.Fatalf("expected <=30 bytes, got %d", len(preview))
	}
}
