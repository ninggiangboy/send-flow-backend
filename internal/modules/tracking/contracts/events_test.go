package contracts

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/platform/events"
)

func TestEmailOpenedPayload_JSONFields(t *testing.T) {
	now := time.Now().UTC().Format(time.RFC3339)
	payload := EmailOpenedPayload{
		TrackingEventID:   "te_1",
		WorkspaceID:       "ws_1",
		MessageID:         "msg_1",
		CampaignID:        "cmp_1",
		Provider:          "fake",
		ProviderMessageID: "prov_msg_1",
		ProviderEventID:   "prov_evt_1",
		NormalizedEventID: "norm_1",
		OccurredAt:        now,
		ReceivedAt:        now,
	}

	envelope, err := events.NewEnvelope(events.NewEnvelopeOptions{
		EventID:       "evt_1",
		EventType:     EventEmailOpenedV1,
		EventVersion:  1,
		AggregateType: "tracking_event",
		AggregateID:   "te_1",
		WorkspaceID:   "ws_1",
		OccurredAt:    time.Now().UTC(),
	}, payload)
	if err != nil {
		t.Fatal(err)
	}

	data, err := events.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}

	var decoded events.Envelope
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	var decodedPayload EmailOpenedPayload
	if err := json.Unmarshal(decoded.Payload, &decodedPayload); err != nil {
		t.Fatal(err)
	}

	if decodedPayload.TrackingEventID != "te_1" {
		t.Errorf("expected tracking_event_id te_1, got %s", decodedPayload.TrackingEventID)
	}
	if decodedPayload.WorkspaceID != "ws_1" {
		t.Errorf("expected workspace_id ws_1, got %s", decodedPayload.WorkspaceID)
	}
	if decodedPayload.MessageID != "msg_1" {
		t.Errorf("expected message_id msg_1, got %s", decodedPayload.MessageID)
	}
	if decodedPayload.Provider != "fake" {
		t.Errorf("expected provider fake, got %s", decodedPayload.Provider)
	}
	if decodedPayload.OccurredAt != now {
		t.Errorf("expected occurred_at %s, got %s", now, decodedPayload.OccurredAt)
	}
}

func TestLinkClickedPayload_JSONFields(t *testing.T) {
	now := time.Now().UTC().Format(time.RFC3339)
	payload := LinkClickedPayload{
		TrackingEventID:   "te_1",
		TrackingLinkID:    "tl_1",
		WorkspaceID:       "ws_1",
		MessageID:         "msg_1",
		DestinationURL:    "https://example.com",
		Provider:          "fake",
		ProviderMessageID: "prov_msg_1",
		OccurredAt:        now,
		ReceivedAt:        now,
	}

	envelope, err := events.NewEnvelope(events.NewEnvelopeOptions{
		EventID:       "evt_1",
		EventType:     EventLinkClickedV1,
		EventVersion:  1,
		AggregateType: "tracking_event",
		AggregateID:   "te_1",
		WorkspaceID:   "ws_1",
		OccurredAt:    time.Now().UTC(),
	}, payload)
	if err != nil {
		t.Fatal(err)
	}

	data, err := events.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}

	var envelope2 events.Envelope
	if err := json.Unmarshal(data, &envelope2); err != nil {
		t.Fatal(err)
	}

	var decodedPayload LinkClickedPayload
	if err := json.Unmarshal(envelope2.Payload, &decodedPayload); err != nil {
		t.Fatal(err)
	}

	if decodedPayload.TrackingEventID != "te_1" {
		t.Errorf("expected tracking_event_id te_1, got %s", decodedPayload.TrackingEventID)
	}
	if decodedPayload.DestinationURL != "https://example.com" {
		t.Errorf("expected destination_url https://example.com, got %s", decodedPayload.DestinationURL)
	}
	if decodedPayload.WorkspaceID != "ws_1" {
		t.Errorf("expected workspace_id ws_1, got %s", decodedPayload.WorkspaceID)
	}
}

func TestRecipientUnsubscribedPayload_JSONFields(t *testing.T) {
	now := time.Now().UTC().Format(time.RFC3339)
	payload := RecipientUnsubscribedPayload{
		TrackingEventID: "te_1",
		SuppressionID:   "sup_1",
		WorkspaceID:     "ws_1",
		MessageID:       "msg_1",
		CampaignID:      "cmp_1",
		Source:          "http",
		OccurredAt:      now,
		ReceivedAt:      now,
	}

	envelope, err := events.NewEnvelope(events.NewEnvelopeOptions{
		EventID:       "evt_1",
		EventType:     EventRecipientUnsubscribedV1,
		EventVersion:  1,
		AggregateType: "tracking_event",
		AggregateID:   "te_1",
		WorkspaceID:   "ws_1",
		OccurredAt:    time.Now().UTC(),
	}, payload)
	if err != nil {
		t.Fatal(err)
	}

	data, err := events.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}

	var envelope2 events.Envelope
	if err := json.Unmarshal(data, &envelope2); err != nil {
		t.Fatal(err)
	}

	var decodedPayload RecipientUnsubscribedPayload
	if err := json.Unmarshal(envelope2.Payload, &decodedPayload); err != nil {
		t.Fatal(err)
	}

	if decodedPayload.TrackingEventID != "te_1" {
		t.Errorf("expected tracking_event_id te_1, got %s", decodedPayload.TrackingEventID)
	}
	if decodedPayload.SuppressionID != "sup_1" {
		t.Errorf("expected suppression_id sup_1, got %s", decodedPayload.SuppressionID)
	}
	if decodedPayload.Source != "http" {
		t.Errorf("expected source http, got %s", decodedPayload.Source)
	}
}
