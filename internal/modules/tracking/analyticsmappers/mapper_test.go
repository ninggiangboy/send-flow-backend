package analyticsmappers

import (
	"testing"
	"time"

	analyticsapp "github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/app"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/domain"
	trackingcontracts "github.com/ninggiangboy/send-flow/backend/internal/modules/tracking/contracts"
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

func TestMapEmailOpened(t *testing.T) {
	payload := trackingcontracts.EmailOpenedPayload{
		TrackingEventID: "trk_1",
		WorkspaceID:     "ws_1",
		MessageID:       "msg_1",
		CampaignID:      "camp_1",
		Provider:        "ses",
		OccurredAt:      "2024-01-15T10:00:00Z",
		ReceivedAt:      "2024-01-15T10:00:05Z",
	}
	env := newTestEnvelope(trackingcontracts.EventEmailOpenedV1, payload)

	mapped, err := MapEmailOpened(env)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mapped.Input.CanonicalType != domain.EventTypeOpened {
		t.Fatalf("expected opened, got %s", mapped.Input.CanonicalType)
	}
}

func TestMapLinkClicked(t *testing.T) {
	payload := trackingcontracts.LinkClickedPayload{
		TrackingEventID: "trk_1",
		WorkspaceID:     "ws_1",
		MessageID:       "msg_1",
		CampaignID:      "camp_1",
		OccurredAt:      "2024-01-15T10:00:00Z",
		ReceivedAt:      "2024-01-15T10:00:05Z",
	}
	env := newTestEnvelope(trackingcontracts.EventLinkClickedV1, payload)

	mapped, err := MapLinkClicked(env)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mapped.Input.CanonicalType != domain.EventTypeClicked {
		t.Fatalf("expected clicked, got %s", mapped.Input.CanonicalType)
	}
}

func TestMapRecipientUnsubscribed(t *testing.T) {
	payload := trackingcontracts.RecipientUnsubscribedPayload{
		WorkspaceID: "ws_1",
		CampaignID:  "camp_1",
		MessageID:   "msg_1",
		Source:      "list_unsubscribe",
		OccurredAt:  "2024-01-15T10:00:00Z",
		ReceivedAt:  "2024-01-15T10:00:05Z",
	}
	env := newTestEnvelope(trackingcontracts.EventRecipientUnsubscribedV1, payload)

	mapped, err := MapRecipientUnsubscribed(env)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mapped.Input.CanonicalType != domain.EventTypeUnsubscribed {
		t.Fatalf("expected unsubscribed, got %s", mapped.Input.CanonicalType)
	}
}

func TestTrackingRegisterAll(t *testing.T) {
	r := analyticsapp.NewMapperRegistry()
	RegisterAll(r)

	types := r.EventTypes()
	if len(types) != 3 {
		t.Fatalf("expected 3 tracking mappers, got %d", len(types))
	}
}
