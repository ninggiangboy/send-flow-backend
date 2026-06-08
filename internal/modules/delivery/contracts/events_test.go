package contracts

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/platform/events"
)

func TestMessageDeliveredPayload_JSONFields(t *testing.T) {
	now := time.Now().UTC().Format(time.RFC3339)
	payload := MessageDeliveredPayload{
		MessageID:         "msg_1",
		WorkspaceID:       "ws_1",
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
		EventType:     EventDeliveryMessageDeliveredV1,
		EventVersion:  1,
		AggregateType: "message",
		AggregateID:   "msg_1",
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

	var decodedPayload MessageDeliveredPayload
	if err := json.Unmarshal(decoded.Payload, &decodedPayload); err != nil {
		t.Fatal(err)
	}

	if decodedPayload.MessageID != "msg_1" {
		t.Errorf("expected message_id msg_1, got %s", decodedPayload.MessageID)
	}
	if decodedPayload.WorkspaceID != "ws_1" {
		t.Errorf("expected workspace_id ws_1, got %s", decodedPayload.WorkspaceID)
	}
	if decodedPayload.Provider != "fake" {
		t.Errorf("expected provider fake, got %s", decodedPayload.Provider)
	}
	if decodedPayload.ProviderMessageID != "prov_msg_1" {
		t.Errorf("expected provider_message_id prov_msg_1, got %s", decodedPayload.ProviderMessageID)
	}
	if decodedPayload.NormalizedEventID != "norm_1" {
		t.Errorf("expected normalized_event_id norm_1, got %s", decodedPayload.NormalizedEventID)
	}
}

func TestMessageBouncedPayload_JSONFields(t *testing.T) {
	now := time.Now().UTC().Format(time.RFC3339)
	payload := MessageBouncedPayload{
		MessageID:         "msg_1",
		WorkspaceID:       "ws_1",
		Provider:          "fake",
		ProviderMessageID: "prov_msg_1",
		NormalizedEventID: "norm_1",
		OccurredAt:        now,
		ReceivedAt:        now,
	}

	envelope, err := events.NewEnvelope(events.NewEnvelopeOptions{
		EventID:       "evt_1",
		EventType:     EventDeliveryMessageBouncedV1,
		EventVersion:  1,
		AggregateType: "message",
		AggregateID:   "msg_1",
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

	var decodedPayload MessageBouncedPayload
	if err := json.Unmarshal(envelope2.Payload, &decodedPayload); err != nil {
		t.Fatal(err)
	}

	if decodedPayload.MessageID != "msg_1" {
		t.Errorf("expected message_id msg_1, got %s", decodedPayload.MessageID)
	}
	if decodedPayload.Provider != "fake" {
		t.Errorf("expected provider fake, got %s", decodedPayload.Provider)
	}
}

func TestMessageComplainedPayload_JSONFields(t *testing.T) {
	now := time.Now().UTC().Format(time.RFC3339)
	payload := MessageComplainedPayload{
		MessageID:         "msg_1",
		WorkspaceID:       "ws_1",
		Provider:          "fake",
		ProviderMessageID: "prov_msg_1",
		NormalizedEventID: "norm_1",
		OccurredAt:        now,
		ReceivedAt:        now,
	}

	envelope, err := events.NewEnvelope(events.NewEnvelopeOptions{
		EventID:       "evt_1",
		EventType:     EventDeliveryMessageComplainedV1,
		EventVersion:  1,
		AggregateType: "message",
		AggregateID:   "msg_1",
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

	var decodedPayload MessageComplainedPayload
	if err := json.Unmarshal(envelope2.Payload, &decodedPayload); err != nil {
		t.Fatal(err)
	}

	if decodedPayload.MessageID != "msg_1" {
		t.Errorf("expected message_id msg_1, got %s", decodedPayload.MessageID)
	}
	if decodedPayload.Provider != "fake" {
		t.Errorf("expected provider fake, got %s", decodedPayload.Provider)
	}
}

func TestSuppressionRecipientSuppressedPayload_JSONFields(t *testing.T) {
	now := time.Now().UTC().Format(time.RFC3339)
	payload := SuppressionRecipientSuppressedPayload{
		SuppressionID:   "sup_1",
		WorkspaceID:     "ws_1",
		EmailNormalized: "test@example.com",
		Scope:           "workspace",
		Reason:          "bounce",
		Source:          "provider_event",
		SourceEventID:   "norm_1",
		CreatedAt:       now,
	}

	envelope, err := events.NewEnvelope(events.NewEnvelopeOptions{
		EventID:       "evt_1",
		EventType:     EventSuppressionRecipientSuppressedV1,
		EventVersion:  1,
		AggregateType: "suppression_entry",
		AggregateID:   "sup_1",
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

	var decodedPayload SuppressionRecipientSuppressedPayload
	if err := json.Unmarshal(envelope2.Payload, &decodedPayload); err != nil {
		t.Fatal(err)
	}

	if decodedPayload.SuppressionID != "sup_1" {
		t.Errorf("expected suppression_id sup_1, got %s", decodedPayload.SuppressionID)
	}
	if decodedPayload.EmailNormalized != "test@example.com" {
		t.Errorf("expected email_normalized test@example.com, got %s", decodedPayload.EmailNormalized)
	}
	if decodedPayload.Reason != "bounce" {
		t.Errorf("expected reason bounce, got %s", decodedPayload.Reason)
	}
}
