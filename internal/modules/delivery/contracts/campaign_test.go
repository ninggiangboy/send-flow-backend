package contracts

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestParseValidCampaignScheduledPayload(t *testing.T) {
	now := time.Now().UTC().Format(time.RFC3339)
	payload := CampaignScheduledPayload{
		CampaignID:        uuid.NewString(),
		WorkspaceID:       uuid.NewString(),
		TemplateID:        uuid.NewString(),
		TemplateVersionID: uuid.NewString(),
		SenderDomainID:    uuid.NewString(),
		MessageType:       "marketing",
		ScheduledAt:       now,
		PlannedRecipients: 1000,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}

	result, err := ParseCampaignScheduledPayload(EventCampaignScheduledV1, data)
	if err != nil {
		t.Fatal(err)
	}

	if result.CampaignID != payload.CampaignID {
		t.Errorf("expected CampaignID %q, got %q", payload.CampaignID, result.CampaignID)
	}
	if result.WorkspaceID != payload.WorkspaceID {
		t.Errorf("expected WorkspaceID %q, got %q", payload.WorkspaceID, result.WorkspaceID)
	}
	if result.TemplateID != payload.TemplateID {
		t.Errorf("expected TemplateID %q, got %q", payload.TemplateID, result.TemplateID)
	}
	if result.TemplateVersionID != payload.TemplateVersionID {
		t.Errorf("expected TemplateVersionID %q, got %q", payload.TemplateVersionID, result.TemplateVersionID)
	}
	if result.SenderDomainID != payload.SenderDomainID {
		t.Errorf("expected SenderDomainID %q, got %q", payload.SenderDomainID, result.SenderDomainID)
	}
	if result.MessageType != payload.MessageType {
		t.Errorf("expected MessageType %q, got %q", payload.MessageType, result.MessageType)
	}
	if result.ScheduledAt != payload.ScheduledAt {
		t.Errorf("expected ScheduledAt %q, got %q", payload.ScheduledAt, result.ScheduledAt)
	}
	if result.PlannedRecipients != payload.PlannedRecipients {
		t.Errorf("expected PlannedRecipients %d, got %d", payload.PlannedRecipients, result.PlannedRecipients)
	}
}

func TestParseWrongEventType(t *testing.T) {
	payload := CampaignScheduledPayload{
		CampaignID:  uuid.NewString(),
		WorkspaceID: uuid.NewString(),
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}

	_, err = ParseCampaignScheduledPayload("some.other.event", data)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrUnsupportedEventType) {
		t.Errorf("expected ErrUnsupportedEventType, got %v", err)
	}
}

func TestParseMissingCampaignID(t *testing.T) {
	payload := CampaignScheduledPayload{
		WorkspaceID: uuid.NewString(),
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}

	_, err = ParseCampaignScheduledPayload(EventCampaignScheduledV1, data)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrInvalidPayload) {
		t.Errorf("expected ErrInvalidPayload, got %v", err)
	}
}

func TestParseMissingWorkspaceID(t *testing.T) {
	payload := CampaignScheduledPayload{
		CampaignID: uuid.NewString(),
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}

	_, err = ParseCampaignScheduledPayload(EventCampaignScheduledV1, data)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrInvalidPayload) {
		t.Errorf("expected ErrInvalidPayload, got %v", err)
	}
}

func TestParseInvalidScheduledAt(t *testing.T) {
	payload := CampaignScheduledPayload{
		CampaignID:        uuid.NewString(),
		WorkspaceID:       uuid.NewString(),
		TemplateID:        uuid.NewString(),
		TemplateVersionID: uuid.NewString(),
		SenderDomainID:    uuid.NewString(),
		MessageType:       "marketing",
		ScheduledAt:       "not-a-timestamp",
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}

	_, err = ParseCampaignScheduledPayload(EventCampaignScheduledV1, data)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrInvalidScheduledAt) {
		t.Errorf("expected ErrInvalidScheduledAt, got %v", err)
	}
}
