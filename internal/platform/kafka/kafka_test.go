package kafka

import (
	"testing"

	"github.com/ninggiangboy/send-flow/backend/internal/platform/events"
	segmentio "github.com/segmentio/kafka-go"
)

func TestBrokers_TrimsEmptyValues(t *testing.T) {
	got := Brokers(" localhost:9092, ,localhost:9093 ")
	if len(got) != 2 {
		t.Fatalf("expected 2 brokers, got %d", len(got))
	}
	if got[0] != "localhost:9092" || got[1] != "localhost:9093" {
		t.Fatalf("unexpected brokers: %#v", got)
	}
}

func TestNewMessageFromEnvelope_UsesAggregateIDAsKey(t *testing.T) {
	env, err := events.NewEnvelope(events.NewEnvelopeOptions{
		EventID:       "evt_123",
		EventType:     "identity.user.registered.v1",
		EventVersion:  1,
		AggregateType: "user",
		AggregateID:   "user_123",
	}, map[string]string{"email": "a@example.com"})
	if err != nil {
		t.Fatalf("unexpected envelope error: %v", err)
	}

	msg, err := NewMessageFromEnvelope(env)
	if err != nil {
		t.Fatalf("unexpected message error: %v", err)
	}
	if msg.Topic != "identity.user.registered" {
		t.Fatalf("unexpected topic: %s", msg.Topic)
	}
	if string(msg.Key) != "user_123" {
		t.Fatalf("unexpected key: %s", string(msg.Key))
	}
	if msg.Headers["event_type"] != env.EventType {
		t.Fatalf("missing event_type header")
	}
}

func TestFromSegmentMessage_MapsHeaders(t *testing.T) {
	msg := fromSegmentMessage(segmentio.Message{
		Topic: "identity.user.registered",
		Key:   []byte("user_123"),
		Value: []byte(`{"ok":true}`),
		Headers: []segmentio.Header{
			{Key: "event_id", Value: []byte("evt_123")},
			{Key: "event_type", Value: []byte("identity.user.registered.v1")},
		},
	})

	if msg.Topic != "identity.user.registered" {
		t.Fatalf("unexpected topic: %s", msg.Topic)
	}
	if string(msg.Key) != "user_123" {
		t.Fatalf("unexpected key: %s", string(msg.Key))
	}
	if msg.Headers["event_id"] != "evt_123" {
		t.Fatalf("unexpected event id header: %#v", msg.Headers)
	}
}
