package contracts

const (
	EventCampaignScheduledV1                 = "campaign.scheduled.v1"
	EventDeliveryMessageQueuedV1             = "delivery.message.queued.v1"
	EventDeliveryMessageAcceptedV1           = "delivery.message.accepted.v1"
	EventDeliveryMessageDeliveredV1          = "delivery.message.delivered.v1"
	EventDeliveryMessageBouncedV1            = "delivery.message.bounced.v1"
	EventDeliveryMessageComplainedV1         = "delivery.message.complained.v1"
	EventDeliveryMessageRetryScheduledV1     = "delivery.message.retry_scheduled.v1"
	EventDeliveryTransactionalSendAcceptedV1 = "delivery.transactional_send.accepted.v1"
)

type MessageQueuedPayload struct {
	MessageID              string `json:"message_id"`
	WorkspaceID            string `json:"workspace_id"`
	CampaignID             string `json:"campaign_id,omitempty"`
	CampaignCandidateID    string `json:"campaign_candidate_id,omitempty"`
	TransactionalRequestID string `json:"transactional_request_id,omitempty"`
	TemplateID             string `json:"template_id"`
	TemplateVersionID      string `json:"template_version_id"`
	SenderDomainID         string `json:"sender_domain_id"`
	MessageType            string `json:"message_type"`
	SourceType             string `json:"source_type"`
	ScheduledAt            string `json:"scheduled_at,omitempty"`
}

type MessageAcceptedPayload struct {
	MessageID              string `json:"message_id"`
	WorkspaceID            string `json:"workspace_id"`
	CampaignID             string `json:"campaign_id"`
	CampaignCandidateID    string `json:"campaign_candidate_id"`
	TransactionalRequestID string `json:"transactional_request_id"`
	TemplateID             string `json:"template_id"`
	TemplateVersionID      string `json:"template_version_id"`
	SenderDomainID         string `json:"sender_domain_id"`
	MessageType            string `json:"message_type"`
	SourceType             string `json:"source_type"`
	Provider               string `json:"provider"`
	ProviderMessageID      string `json:"provider_message_id"`
	AcceptedAt             string `json:"accepted_at"`
}
