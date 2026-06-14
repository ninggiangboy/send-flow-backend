package contracts

const (
	EventConfigCreatedV1          = "webhooks.config.created.v1"
	EventConfigUpdatedV1          = "webhooks.config.updated.v1"
	EventConfigDisabledV1         = "webhooks.config.disabled.v1"
	EventSecretRotatedV1          = "webhooks.secret.rotated.v1"
	EventDeliverySucceededV1      = "webhooks.delivery.succeeded.v1"
	EventDeliveryFailedV1         = "webhooks.delivery.failed.v1"
	EventDeliveryRetryScheduledV1 = "webhooks.delivery.retry_scheduled.v1"
	EventDueDeliveriesProcessV1   = "webhooks.due_deliveries.process.v1"
)

type DueDeliveriesProcessPayload struct {
	Limit int    `json:"limit"`
	Now   string `json:"now"`
}

type ConfigCreatedPayload struct {
	ConfigID    string   `json:"config_id"`
	WorkspaceID string   `json:"workspace_id"`
	Name        string   `json:"name"`
	TargetURL   string   `json:"target_url"`
	Status      string   `json:"status"`
	Subscribed  []string `json:"subscribed"`
}

type ConfigUpdatedPayload struct {
	ConfigID    string   `json:"config_id"`
	WorkspaceID string   `json:"workspace_id"`
	Version     int64    `json:"version"`
	Status      string   `json:"status"`
	Subscribed  []string `json:"subscribed"`
}

type ConfigDisabledPayload struct {
	ConfigID    string `json:"config_id"`
	WorkspaceID string `json:"workspace_id"`
	DisabledAt  string `json:"disabled_at"`
}

type SecretRotatedPayload struct {
	ConfigID    string `json:"config_id"`
	WorkspaceID string `json:"workspace_id"`
	Version     int64  `json:"version"`
	SecretHint  string `json:"secret_hint"`
}

type DeliverySucceededPayload struct {
	DeliveryID      string `json:"delivery_id"`
	WorkspaceID     string `json:"workspace_id"`
	WebhookID       string `json:"webhook_id"`
	SourceEventID   string `json:"source_event_id"`
	SourceEventType string `json:"source_event_type"`
	StatusCode      int    `json:"status_code"`
	DurationMs      int64  `json:"duration_ms"`
}

type DeliveryFailedPayload struct {
	DeliveryID      string `json:"delivery_id"`
	WorkspaceID     string `json:"workspace_id"`
	WebhookID       string `json:"webhook_id"`
	SourceEventID   string `json:"source_event_id"`
	SourceEventType string `json:"source_event_type"`
	StatusCode      *int   `json:"status_code,omitempty"`
	Error           string `json:"error"`
	DurationMs      int64  `json:"duration_ms"`
}

type DeliveryRetryScheduledPayload struct {
	DeliveryID      string `json:"delivery_id"`
	WorkspaceID     string `json:"workspace_id"`
	WebhookID       string `json:"webhook_id"`
	SourceEventID   string `json:"source_event_id"`
	SourceEventType string `json:"source_event_type"`
	NextAttemptAt   string `json:"next_attempt_at"`
	AttemptNumber   int64  `json:"attempt_number"`
	StatusCode      *int   `json:"status_code,omitempty"`
	Error           string `json:"error"`
}

type CustomerWebhookPayload struct {
	ID          string         `json:"id"`
	Type        string         `json:"type"`
	WorkspaceID string         `json:"workspace_id"`
	OccurredAt  string         `json:"occurred_at"`
	Data        map[string]any `json:"data"`
	Attempt     *int           `json:"attempt,omitempty"`
}
