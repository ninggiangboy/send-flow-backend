package contracts

const (
	EventRecipientSuppressedV1 = "suppression.recipient_suppressed.v1"
)

type RecipientSuppressedPayload struct {
	SuppressionID   string `json:"suppression_id"`
	WorkspaceID     string `json:"workspace_id"`
	Email           string `json:"email"`
	EmailNormalized string `json:"email_normalized"`
	Scope           string `json:"scope"`
	Reason          string `json:"reason"`
	Source          string `json:"source"`
	SourceEventID   string `json:"source_event_id,omitempty"`
	SuppressedAt    string `json:"suppressed_at"`
}
