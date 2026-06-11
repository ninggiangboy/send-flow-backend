package contracts

const (
	EventAuditEntryRecordedV1 = "audit.entry.recorded.v1"
)

type AuditEntryRecordedPayload struct {
	EntryID     string `json:"entry_id"`
	WorkspaceID string `json:"workspace_id"`
	ActorUserID string `json:"actor_user_id,omitempty"`
	ActionType  string `json:"action_type"`
	TargetType  string `json:"target_type,omitempty"`
	TargetID    string `json:"target_id,omitempty"`
	OccurredAt  string `json:"occurred_at"`
}
