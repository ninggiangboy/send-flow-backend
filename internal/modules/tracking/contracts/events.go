package contracts

const (
	EventEmailOpenedV1           = "tracking.email_opened.v1"
	EventLinkClickedV1           = "tracking.link_clicked.v1"
	EventRecipientUnsubscribedV1 = "tracking.recipient_unsubscribed.v1"
)

const AggregateTrackingEvent = "tracking_event"

type EmailOpenedPayload struct {
	TrackingEventID   string `json:"tracking_event_id"`
	WorkspaceID       string `json:"workspace_id"`
	MessageID         string `json:"message_id"`
	CampaignID        string `json:"campaign_id,omitempty"`
	Provider          string `json:"provider,omitempty"`
	ProviderMessageID string `json:"provider_message_id,omitempty"`
	ProviderEventID   string `json:"provider_event_id,omitempty"`
	NormalizedEventID string `json:"normalized_event_id,omitempty"`
	OccurredAt        string `json:"occurred_at"`
	ReceivedAt        string `json:"received_at"`
}

type LinkClickedPayload struct {
	TrackingEventID   string `json:"tracking_event_id"`
	TrackingLinkID    string `json:"tracking_link_id,omitempty"`
	WorkspaceID       string `json:"workspace_id"`
	MessageID         string `json:"message_id"`
	CampaignID        string `json:"campaign_id,omitempty"`
	DestinationURL    string `json:"destination_url,omitempty"`
	Provider          string `json:"provider,omitempty"`
	ProviderMessageID string `json:"provider_message_id,omitempty"`
	ProviderEventID   string `json:"provider_event_id,omitempty"`
	NormalizedEventID string `json:"normalized_event_id,omitempty"`
	OccurredAt        string `json:"occurred_at"`
	ReceivedAt        string `json:"received_at"`
}

type RecipientUnsubscribedPayload struct {
	TrackingEventID string `json:"tracking_event_id,omitempty"`
	SuppressionID   string `json:"suppression_id,omitempty"`
	WorkspaceID     string `json:"workspace_id"`
	MessageID       string `json:"message_id,omitempty"`
	CampaignID      string `json:"campaign_id,omitempty"`
	Source          string `json:"source"`
	SourceEventID   string `json:"source_event_id,omitempty"`
	OccurredAt      string `json:"occurred_at"`
	ReceivedAt      string `json:"received_at"`
}
