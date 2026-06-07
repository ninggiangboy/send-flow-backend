package contracts

import (
	"errors"
	"fmt"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/platform/events"
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

func ParseCampaignScheduledPayload(data []byte) (*CampaignScheduledPayload, error) {
	envelope, err := events.Unmarshal(data)
	if err != nil {
		return nil, fmt.Errorf("unmarshal envelope: %w", err)
	}

	if envelope.EventType != EventCampaignScheduledV1 {
		return nil, fmt.Errorf("expected %q, got %q: %w", EventCampaignScheduledV1, envelope.EventType, ErrUnsupportedEventType)
	}

	var payload CampaignScheduledPayload
	if err := envelope.DecodePayload(&payload); err != nil {
		return nil, fmt.Errorf("decode payload: %w", err)
	}

	if payload.CampaignID == "" {
		return nil, fmt.Errorf("campaign_id: %w", ErrInvalidPayload)
	}
	if payload.WorkspaceID == "" {
		return nil, fmt.Errorf("workspace_id: %w", ErrInvalidPayload)
	}
	if payload.TemplateID == "" {
		return nil, fmt.Errorf("template_id: %w", ErrInvalidPayload)
	}
	if payload.TemplateVersionID == "" {
		return nil, fmt.Errorf("template_version_id: %w", ErrInvalidPayload)
	}
	if payload.SenderDomainID == "" {
		return nil, fmt.Errorf("sender_domain_id: %w", ErrInvalidPayload)
	}
	if payload.MessageType == "" {
		return nil, fmt.Errorf("message_type: %w", ErrInvalidPayload)
	}

	if _, err := time.Parse(time.RFC3339, payload.ScheduledAt); err != nil {
		return nil, fmt.Errorf("%q: %w", payload.ScheduledAt, ErrInvalidScheduledAt)
	}

	return &payload, nil
}
