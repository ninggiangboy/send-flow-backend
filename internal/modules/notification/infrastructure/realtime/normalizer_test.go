package realtime

import (
	"testing"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/notification/contracts"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/events"
)

func mustEnvelope(t *testing.T, eventType string, payload any) events.Envelope {
	t.Helper()
	env, err := events.NewEnvelope(events.NewEnvelopeOptions{
		EventID:       "evt-1",
		EventType:     eventType,
		EventVersion:  1,
		AggregateType: "notification_message",
		AggregateID:   "msg-1",
		WorkspaceID:   "ws-1",
		OccurredAt:    time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC),
	}, payload)
	if err != nil {
		t.Fatal(err)
	}
	return env
}

func TestNormalizeEnvelope_Queued(t *testing.T) {
	payload := contracts.MessageQueuedPayload{
		MessageID:       "msg-1",
		WorkspaceID:     "ws-1",
		Type:            "welcome_email",
		RecipientEmail:  "user@example.com",
		RecipientUserID: "user-1",
		Status:          "queued",
		CreatedAt:       "2026-06-15T12:00:00Z",
	}
	env := mustEnvelope(t, contracts.EventMessageQueuedV1, payload)

	ev, ok, err := NormalizeEnvelope(env)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected ok=true")
	}

	if ev.EventID != "evt-1" {
		t.Errorf("EventID = %q, want evt-1", ev.EventID)
	}
	if ev.EventType != contracts.EventMessageQueuedV1 {
		t.Errorf("EventType = %q, want %q", ev.EventType, contracts.EventMessageQueuedV1)
	}
	if ev.MessageID != "msg-1" {
		t.Errorf("MessageID = %q, want msg-1", ev.MessageID)
	}
	if ev.WorkspaceID != "ws-1" {
		t.Errorf("WorkspaceID = %q, want ws-1", ev.WorkspaceID)
	}
	if ev.RecipientUserID != "user-1" {
		t.Errorf("RecipientUserID = %q, want user-1", ev.RecipientUserID)
	}
	if ev.Type != "welcome_email" {
		t.Errorf("Type = %q, want welcome_email", ev.Type)
	}
	if ev.Status != "queued" {
		t.Errorf("Status = %q, want queued", ev.Status)
	}
	if ev.AttemptNumber != nil {
		t.Error("AttemptNumber should be nil for queued events")
	}
	if ev.Provider != "" {
		t.Errorf("Provider = %q, want empty", ev.Provider)
	}
	if ev.FinalFailure != nil {
		t.Error("FinalFailure should be nil for queued events")
	}
}

func TestNormalizeEnvelope_Sent(t *testing.T) {
	payload := contracts.MessageSentPayload{
		MessageID:       "msg-1",
		WorkspaceID:     "ws-1",
		Type:            "welcome_email",
		RecipientEmail:  "user@example.com",
		RecipientUserID: "user-1",
		AttemptNumber:   2,
		Provider:        "smtp",
		SentAt:          "2026-06-15T12:00:00Z",
	}
	env := mustEnvelope(t, contracts.EventMessageSentV1, payload)

	ev, ok, err := NormalizeEnvelope(env)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected ok=true")
	}

	if ev.Status != "sent" {
		t.Errorf("Status = %q, want sent", ev.Status)
	}
	if ev.AttemptNumber == nil || *ev.AttemptNumber != 2 {
		t.Errorf("AttemptNumber = %v, want 2", ev.AttemptNumber)
	}
	if ev.Provider != "smtp" {
		t.Errorf("Provider = %q, want smtp", ev.Provider)
	}
	if ev.FinalFailure != nil {
		t.Error("FinalFailure should be nil for sent events")
	}
}

func TestNormalizeEnvelope_Failed(t *testing.T) {
	final := true
	payload := contracts.MessageFailedPayload{
		MessageID:       "msg-1",
		WorkspaceID:     "ws-1",
		Type:            "welcome_email",
		RecipientEmail:  "user@example.com",
		RecipientUserID: "user-1",
		AttemptNumber:   3,
		ErrorMessage:    "connection refused",
		FinalFailure:    final,
	}
	env := mustEnvelope(t, contracts.EventMessageFailedV1, payload)

	ev, ok, err := NormalizeEnvelope(env)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected ok=true")
	}

	if ev.Status != "failed" {
		t.Errorf("Status = %q, want failed", ev.Status)
	}
	if ev.AttemptNumber == nil || *ev.AttemptNumber != 3 {
		t.Errorf("AttemptNumber = %v, want 3", ev.AttemptNumber)
	}
	if ev.FinalFailure == nil || *ev.FinalFailure != true {
		t.Error("FinalFailure should be true")
	}
	// Ensure error_message is not leaked
	if ev.Provider != "" {
		t.Errorf("Provider = %q, want empty", ev.Provider)
	}
}

func TestNormalizeEnvelope_NonNotification(t *testing.T) {
	env := mustEnvelope(t, "some.other.event.v1", map[string]string{"foo": "bar"})
	_, ok, err := NormalizeEnvelope(env)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected ok=false for non-notification event")
	}
}

func TestNormalizeEnvelope_MalformedPayload(t *testing.T) {
	env := mustEnvelope(t, contracts.EventMessageQueuedV1, "not-a-struct")
	_, _, err := NormalizeEnvelope(env)
	if err == nil {
		t.Fatal("expected error for malformed payload")
	}
}

func TestNormalizeEnvelope_FallbackToEnvelope(t *testing.T) {
	// When payload fields are empty, envelope values should be used as fallback
	payload := contracts.MessageQueuedPayload{
		Type:      "welcome_email",
		Status:    "queued",
		CreatedAt: "2026-06-15T12:00:00Z",
	}
	env := mustEnvelope(t, contracts.EventMessageQueuedV1, payload)

	ev, ok, err := NormalizeEnvelope(env)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected ok=true")
	}
	if ev.MessageID != "msg-1" {
		t.Errorf("MessageID should fallback to envelope AggregateID, got %q", ev.MessageID)
	}
	if ev.WorkspaceID != "ws-1" {
		t.Errorf("WorkspaceID should fallback to envelope WorkspaceID, got %q", ev.WorkspaceID)
	}
}

func TestNormalizeEnvelope_RecipientUserIDOmitted(t *testing.T) {
	payload := contracts.MessageQueuedPayload{
		MessageID: "msg-1",
		Type:      "welcome_email",
		Status:    "queued",
		CreatedAt: "2026-06-15T12:00:00Z",
	}
	env := mustEnvelope(t, contracts.EventMessageQueuedV1, payload)

	ev, ok, err := NormalizeEnvelope(env)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected ok=true")
	}
	if ev.RecipientUserID != "" {
		t.Errorf("RecipientUserID should be empty when not in payload, got %q", ev.RecipientUserID)
	}
}
