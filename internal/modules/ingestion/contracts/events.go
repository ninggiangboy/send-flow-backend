package contracts

const (
	EventProviderWebhookReceivedV1 = "ingestion.provider_webhook.received.v1"
	EventProviderEventNormalizedV1 = "ingestion.provider_event.normalized.v1"
)

const (
	AggregateProviderWebhookEvent = "provider_webhook_event"
	AggregateNormalizedEvent      = "normalized_provider_event"
)

type ProviderWebhookReceivedPayload struct {
	RawEventID        string `json:"raw_event_id"`
	Provider          string `json:"provider"`
	ProviderEventID   string `json:"provider_event_id,omitempty"`
	ProviderMessageID string `json:"provider_message_id,omitempty"`
	WorkspaceID       string `json:"workspace_id,omitempty"`
	MessageID         string `json:"message_id,omitempty"`
	ReceivedAt        string `json:"received_at"`
}

type ProviderEventNormalizedPayload struct {
	NormalizedEventID string `json:"normalized_event_id"`
	RawEventID        string `json:"raw_event_id"`
	Provider          string `json:"provider"`
	ProviderEventID   string `json:"provider_event_id,omitempty"`
	ProviderMessageID string `json:"provider_message_id,omitempty"`
	WorkspaceID       string `json:"workspace_id,omitempty"`
	MessageID         string `json:"message_id,omitempty"`
	EventType         string `json:"event_type"`
	OccurredAt        string `json:"occurred_at"`
	ReceivedAt        string `json:"received_at"`
}
