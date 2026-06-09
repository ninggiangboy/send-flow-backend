package domain

import "time"

type AuditEntry struct {
	ID             string         `json:"id"`
	WorkspaceID    string         `json:"workspace_id"`
	ActorUserID    string         `json:"actor_user_id,omitempty"`
	ActionType     string         `json:"action_type"`
	TargetType     string         `json:"target_type,omitempty"`
	TargetID       string         `json:"target_id,omitempty"`
	PayloadSummary map[string]any `json:"payload_summary,omitempty"`
	RequestID      string         `json:"request_id,omitempty"`
	OccurredAt     time.Time      `json:"occurred_at"`
}

type AuditFilter struct {
	WorkspaceID string
	ActorUserID string
	ActionType  string
	TargetType  string
	TargetID    string
	From        *time.Time
	To          *time.Time
	Limit       int
	Cursor      string
}
