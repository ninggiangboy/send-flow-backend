package contracts

import (
	"encoding/json"
	"testing"
)

func TestProjectionUpdatedPayloadJSONFields(t *testing.T) {
	now := "2024-06-10T12:00:00Z"
	payload := ProjectionUpdatedPayload{
		WorkspaceID:    "ws_1",
		ProjectionType: "workspace_overview",
		ProjectionID:   "ws_1",
		EventType:      "delivered",
		LastEventAt:    now,
		LastUpdatedAt:  now,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var decoded ProjectionUpdatedPayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if decoded.WorkspaceID != "ws_1" {
		t.Fatalf("expected ws_1, got %s", decoded.WorkspaceID)
	}
	if decoded.ProjectionType != "workspace_overview" {
		t.Fatalf("expected workspace_overview, got %s", decoded.ProjectionType)
	}
	if decoded.ProjectionID != "ws_1" {
		t.Fatalf("expected ws_1, got %s", decoded.ProjectionID)
	}
	if decoded.EventType != "delivered" {
		t.Fatalf("expected delivered, got %s", decoded.EventType)
	}
}

func TestEventFactRecordedPayloadJSONFields(t *testing.T) {
	now := "2024-06-10T12:00:00Z"
	payload := EventFactRecordedPayload{
		FactID:          "fact_1",
		SourceEventID:   "src_1",
		SourceEventType: "delivery.message.delivered.v1",
		WorkspaceID:     "ws_1",
		CampaignID:      "camp_1",
		EventType:       "delivered",
		OccurredAt:      now,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var decoded EventFactRecordedPayload
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if decoded.FactID != "fact_1" {
		t.Fatalf("expected fact_1, got %s", decoded.FactID)
	}
	if decoded.SourceEventID != "src_1" {
		t.Fatalf("expected src_1, got %s", decoded.SourceEventID)
	}
	if decoded.WorkspaceID != "ws_1" {
		t.Fatalf("expected ws_1, got %s", decoded.WorkspaceID)
	}
	if decoded.EventType != "delivered" {
		t.Fatalf("expected delivered, got %s", decoded.EventType)
	}
}

func TestEventConstants(t *testing.T) {
	if EventProjectionUpdatedV1 != "analytics.projection.updated.v1" {
		t.Fatalf("unexpected constant: %s", EventProjectionUpdatedV1)
	}
	if EventEventFactRecordedV1 != "analytics.event_fact.recorded.v1" {
		t.Fatalf("unexpected constant: %s", EventEventFactRecordedV1)
	}
}
