package contracts

const (
	EventProjectionUpdatedV1 = "analytics.projection.updated.v1"
	EventEventFactRecordedV1 = "analytics.event_fact.recorded.v1"
)

type ProjectionUpdatedPayload struct {
	WorkspaceID    string `json:"workspace_id"`
	ProjectionType string `json:"projection_type"`
	ProjectionID   string `json:"projection_id"`
	EventType      string `json:"event_type"`
	LastEventAt    string `json:"last_event_at"`
	LastUpdatedAt  string `json:"last_updated_at"`
}

type EventFactRecordedPayload struct {
	FactID          string `json:"fact_id"`
	SourceEventID   string `json:"source_event_id"`
	SourceEventType string `json:"source_event_type"`
	WorkspaceID     string `json:"workspace_id"`
	CampaignID      string `json:"campaign_id,omitempty"`
	EventType       string `json:"event_type"`
	OccurredAt      string `json:"occurred_at"`
}
