package realtime

import "time"

type Event struct {
	EventID         string    `json:"event_id"`
	EventType       string    `json:"event_type"`
	MessageID       string    `json:"message_id"`
	WorkspaceID     string    `json:"workspace_id,omitempty"`
	RecipientUserID string    `json:"recipient_user_id,omitempty"`
	Type            string    `json:"type"`
	Status          string    `json:"status"`
	AttemptNumber   *int      `json:"attempt_number,omitempty"`
	Provider        string    `json:"provider,omitempty"`
	FinalFailure    *bool     `json:"final_failure,omitempty"`
	OccurredAt      time.Time `json:"occurred_at"`
}
