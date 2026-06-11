package contracts

import "encoding/json"

const (
	EventCampaignScheduledV1 = "campaign.scheduled.v1"
	EventCampaignCancelledV1 = "campaign.cancelled.v1"
	EventCampaignPausedV1    = "campaign.paused.v1"
	EventCampaignResumedV1   = "campaign.resumed.v1"
)

type CampaignScheduledPayload struct {
	CampaignID        string          `json:"campaign_id"`
	WorkspaceID       string          `json:"workspace_id"`
	AudienceRef       json.RawMessage `json:"audience_ref"`
	TemplateID        string          `json:"template_id"`
	TemplateVersionID string          `json:"template_version_id"`
	SenderDomainID    string          `json:"sender_domain_id"`
	MessageType       string          `json:"message_type"`
	ScheduledAt       string          `json:"scheduled_at"`
	PlannedRecipients int64           `json:"planned_recipients"`
}

type CampaignCancelledPayload struct {
	CampaignID  string `json:"campaign_id"`
	WorkspaceID string `json:"workspace_id"`
	CancelledAt string `json:"cancelled_at"`
}

type CampaignPausedPayload struct {
	CampaignID  string `json:"campaign_id"`
	WorkspaceID string `json:"workspace_id"`
	PausedAt    string `json:"paused_at"`
}

type CampaignResumedPayload struct {
	CampaignID  string `json:"campaign_id"`
	WorkspaceID string `json:"workspace_id"`
	ResumedAt   string `json:"resumed_at"`
}
