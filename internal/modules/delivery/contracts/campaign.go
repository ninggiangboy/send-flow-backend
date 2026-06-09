package contracts

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

var (
	ErrUnsupportedEventType = errors.New("unsupported event type")
	ErrInvalidPayload       = errors.New("invalid payload")
	ErrInvalidScheduledAt   = errors.New("invalid scheduled_at timestamp")
)

type CampaignScheduledPayload struct {
	CampaignID        string `json:"campaign_id"`
	WorkspaceID       string `json:"workspace_id"`
	TemplateID        string `json:"template_id"`
	TemplateVersionID string `json:"template_version_id"`
	SenderDomainID    string `json:"sender_domain_id"`
	MessageType       string `json:"message_type"`
	ScheduledAt       string `json:"scheduled_at"`
	PlannedRecipients int64  `json:"planned_recipients"`
}

func ParseCampaignScheduledPayload(eventType string, payload []byte) (*CampaignScheduledPayload, error) {
	if eventType != EventCampaignScheduledV1 {
		return nil, fmt.Errorf("expected %q, got %q: %w", EventCampaignScheduledV1, eventType, ErrUnsupportedEventType)
	}

	var p CampaignScheduledPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, fmt.Errorf("decode payload: %w", err)
	}

	if p.CampaignID == "" {
		return nil, fmt.Errorf("campaign_id: %w", ErrInvalidPayload)
	}
	if p.WorkspaceID == "" {
		return nil, fmt.Errorf("workspace_id: %w", ErrInvalidPayload)
	}
	if p.TemplateID == "" {
		return nil, fmt.Errorf("template_id: %w", ErrInvalidPayload)
	}
	if p.TemplateVersionID == "" {
		return nil, fmt.Errorf("template_version_id: %w", ErrInvalidPayload)
	}
	if p.SenderDomainID == "" {
		return nil, fmt.Errorf("sender_domain_id: %w", ErrInvalidPayload)
	}
	if p.MessageType == "" {
		return nil, fmt.Errorf("message_type: %w", ErrInvalidPayload)
	}

	if _, err := time.Parse(time.RFC3339, p.ScheduledAt); err != nil {
		return nil, fmt.Errorf("%q: %w", p.ScheduledAt, ErrInvalidScheduledAt)
	}

	return &p, nil
}
