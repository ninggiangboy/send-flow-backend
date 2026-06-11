package app

import (
	"testing"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/platform/events"
)

func newTestEnvelope(eventType string, payload any) events.Envelope {
	env, err := events.NewEnvelope(events.NewEnvelopeOptions{
		EventID:       "test-event-id",
		EventType:     eventType,
		EventVersion:  1,
		AggregateType: "test",
		AggregateID:   "test-aggregate",
		OccurredAt:    time.Now(),
	}, payload)
	if err != nil {
		panic(err)
	}
	return env
}

func TestMapperRegistry_UnsupportedEventType(t *testing.T) {
	r := NewMapperRegistry()
	env := newTestEnvelope("unknown.event.v1", map[string]any{})
	_, err := r.MapEvent(env)
	if err != ErrUnsupportedEventType {
		t.Fatalf("expected ErrUnsupportedEventType, got %v", err)
	}
}

func TestMapperRegistry_EventTypes(t *testing.T) {
	r := NewMapperRegistry()
	r.Register("type.a", func(_ events.Envelope) (*MappedEvent, error) { return nil, nil })
	r.Register("type.b", func(_ events.Envelope) (*MappedEvent, error) { return nil, nil })

	types := r.EventTypes()
	if len(types) != 2 {
		t.Fatalf("expected 2 event types, got %d", len(types))
	}
}

func TestParseTimestamp(t *testing.T) {
	ts, err := ParseTimestamp("2024-01-15T10:00:00Z")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ts.Year() != 2024 {
		t.Fatalf("expected year 2024, got %d", ts.Year())
	}
}

func TestParseTimestamp_Invalid(t *testing.T) {
	_, err := ParseTimestamp("not-a-timestamp")
	if err == nil {
		t.Fatal("expected error for invalid timestamp")
	}
}
