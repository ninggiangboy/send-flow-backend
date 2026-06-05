package events

import (
	"testing"
	"time"
)

func TestEnvelope_MarshalRoundTrip(t *testing.T) {
	occurredAt := time.Date(2026, 5, 31, 10, 0, 0, 0, time.UTC)
	env, err := NewEnvelope(NewEnvelopeOptions{
		EventID:       "evt_123",
		EventType:     "identity.user.registered.v1",
		AggregateType: "user",
		AggregateID:   "user_123",
		WorkspaceID:   "workspace_123",
		OccurredAt:    occurredAt,
	}, map[string]string{"email": "a@example.com"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := Marshal(env)
	if err != nil {
		t.Fatalf("unexpected marshal error: %v", err)
	}
	got, err := Unmarshal(data)
	if err != nil {
		t.Fatalf("unexpected unmarshal error: %v", err)
	}

	if got.EventVersion != 1 {
		t.Fatalf("expected default version 1, got %d", got.EventVersion)
	}
	if got.OccurredAt != occurredAt {
		t.Fatalf("unexpected occurred_at: %s", got.OccurredAt)
	}
	if TopicFromEventType(got.EventType) != "identity.user.registered" {
		t.Fatalf("unexpected topic: %s", TopicFromEventType(got.EventType))
	}
}

func TestEnvelope_ValidateRequiresEventID(t *testing.T) {
	_, err := NewEnvelope(NewEnvelopeOptions{
		EventType:   "identity.user.registered.v1",
		AggregateID: "user_123",
	}, map[string]string{"email": "a@example.com"})
	if err != ErrMissingEventID {
		t.Fatalf("expected ErrMissingEventID, got %v", err)
	}
}
