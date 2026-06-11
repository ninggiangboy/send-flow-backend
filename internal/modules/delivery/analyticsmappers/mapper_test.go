package analyticsmappers

import (
	"testing"
	"time"

	analyticsapp "github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/app"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/domain"
	deliverycontracts "github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/contracts"
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

func TestMapMessageQueued(t *testing.T) {
	payload := deliverycontracts.MessageQueuedPayload{
		MessageID:   "msg_1",
		WorkspaceID: "ws_1",
		CampaignID:  "camp_1",
	}
	env := newTestEnvelope(deliverycontracts.EventDeliveryMessageQueuedV1, payload)

	mapped, err := MapMessageQueued(env)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mapped.Input.CanonicalType != domain.EventTypeQueued {
		t.Fatalf("expected queued, got %s", mapped.Input.CanonicalType)
	}
	if mapped.Input.WorkspaceID != "ws_1" {
		t.Fatalf("expected ws_1, got %s", mapped.Input.WorkspaceID)
	}
	if mapped.Input.CampaignID != "camp_1" {
		t.Fatalf("expected camp_1, got %s", mapped.Input.CampaignID)
	}
	if mapped.Input.MessageID != "msg_1" {
		t.Fatalf("expected msg_1, got %s", mapped.Input.MessageID)
	}
}

func TestMapMessageAccepted(t *testing.T) {
	payload := deliverycontracts.MessageAcceptedPayload{
		MessageID:         "msg_1",
		WorkspaceID:       "ws_1",
		CampaignID:        "camp_1",
		Provider:          "ses",
		ProviderMessageID: "prov_msg_1",
		AcceptedAt:        "2024-01-15T10:00:00Z",
	}
	env := newTestEnvelope(deliverycontracts.EventDeliveryMessageAcceptedV1, payload)

	mapped, err := MapMessageAccepted(env)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mapped.Input.CanonicalType != domain.EventTypeAccepted {
		t.Fatalf("expected accepted, got %s", mapped.Input.CanonicalType)
	}
	if mapped.Input.Provider != "ses" {
		t.Fatalf("expected ses, got %s", mapped.Input.Provider)
	}
}

func TestMapMessageDelivered(t *testing.T) {
	payload := deliverycontracts.MessageDeliveredPayload{
		MessageID:         "msg_1",
		WorkspaceID:       "ws_1",
		CampaignID:        "camp_1",
		Provider:          "ses",
		ProviderMessageID: "prov_msg_1",
		ProviderEventID:   "prov_evt_1",
		OccurredAt:        "2024-01-15T10:00:00Z",
		ReceivedAt:        "2024-01-15T10:00:05Z",
	}
	env := newTestEnvelope(deliverycontracts.EventDeliveryMessageDeliveredV1, payload)

	mapped, err := MapMessageDelivered(env)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mapped.Input.CanonicalType != domain.EventTypeDelivered {
		t.Fatalf("expected delivered, got %s", mapped.Input.CanonicalType)
	}
	if mapped.Input.ProviderEventID != "prov_evt_1" {
		t.Fatalf("expected prov_evt_1, got %s", mapped.Input.ProviderEventID)
	}
}

func TestMapMessageBounced(t *testing.T) {
	payload := deliverycontracts.MessageBouncedPayload{
		MessageID:   "msg_1",
		WorkspaceID: "ws_1",
		CampaignID:  "camp_1",
		Provider:    "ses",
		OccurredAt:  "2024-01-15T10:00:00Z",
		ReceivedAt:  "2024-01-15T10:00:05Z",
	}
	env := newTestEnvelope(deliverycontracts.EventDeliveryMessageBouncedV1, payload)

	mapped, err := MapMessageBounced(env)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mapped.Input.CanonicalType != domain.EventTypeBounced {
		t.Fatalf("expected bounced, got %s", mapped.Input.CanonicalType)
	}
}

func TestMapMessageComplained(t *testing.T) {
	payload := deliverycontracts.MessageComplainedPayload{
		MessageID:   "msg_1",
		WorkspaceID: "ws_1",
		CampaignID:  "camp_1",
		Provider:    "ses",
		OccurredAt:  "2024-01-15T10:00:00Z",
		ReceivedAt:  "2024-01-15T10:00:05Z",
	}
	env := newTestEnvelope(deliverycontracts.EventDeliveryMessageComplainedV1, payload)

	mapped, err := MapMessageComplained(env)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mapped.Input.CanonicalType != domain.EventTypeComplained {
		t.Fatalf("expected complained, got %s", mapped.Input.CanonicalType)
	}
}

func TestMapRetryScheduled(t *testing.T) {
	payload := deliverycontracts.MessageRetryScheduledPayload{
		MessageID:     "msg_1",
		WorkspaceID:   "ws_1",
		RetryCount:    1,
		MaxRetries:    3,
		NextAttemptAt: "2024-01-15T10:05:00Z",
	}
	env := newTestEnvelope(deliverycontracts.EventDeliveryMessageRetryScheduledV1, payload)

	mapped, err := MapRetryScheduled(env)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mapped.Input.CanonicalType != domain.EventTypeRetryScheduled {
		t.Fatalf("expected retry_scheduled, got %s", mapped.Input.CanonicalType)
	}
	if mapped.Input.WorkspaceID != "ws_1" {
		t.Fatalf("expected workspace_id ws_1, got %s", mapped.Input.WorkspaceID)
	}
	if mapped.Input.MessageID != "msg_1" {
		t.Fatalf("expected message_id msg_1, got %s", mapped.Input.MessageID)
	}
}

func TestMapMalformedTimestamp(t *testing.T) {
	payload := deliverycontracts.MessageDeliveredPayload{
		MessageID:   "msg_1",
		WorkspaceID: "ws_1",
		OccurredAt:  "not-a-timestamp",
		ReceivedAt:  "2024-01-15T10:00:05Z",
	}
	env := newTestEnvelope(deliverycontracts.EventDeliveryMessageDeliveredV1, payload)
	_, err := MapMessageDelivered(env)
	if err == nil {
		t.Fatal("expected error for malformed timestamp")
	}
}

func TestRegisterAll(t *testing.T) {
	r := analyticsapp.NewMapperRegistry()
	RegisterAll(r)

	types := r.EventTypes()
	if len(types) != 6 {
		t.Fatalf("expected 6 delivery mappers, got %d", len(types))
	}
}
