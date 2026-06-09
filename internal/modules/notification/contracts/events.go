package contracts

const (
	EventMessageQueuedV1 = "notification.message.queued.v1"
	EventMessageSentV1   = "notification.message.sent.v1"
	EventMessageFailedV1 = "notification.message.failed.v1"
)

type MessageQueuedPayload struct {
	MessageID      string `json:"message_id"`
	WorkspaceID    string `json:"workspace_id,omitempty"`
	Type           string `json:"type"`
	RecipientEmail string `json:"recipient_email"`
	Status         string `json:"status"`
	CreatedAt      string `json:"created_at"`
}

type MessageSentPayload struct {
	MessageID      string `json:"message_id"`
	WorkspaceID    string `json:"workspace_id,omitempty"`
	Type           string `json:"type"`
	RecipientEmail string `json:"recipient_email"`
	AttemptNumber  int    `json:"attempt_number"`
	Provider       string `json:"provider"`
	SentAt         string `json:"sent_at"`
}

type MessageFailedPayload struct {
	MessageID      string `json:"message_id"`
	WorkspaceID    string `json:"workspace_id,omitempty"`
	Type           string `json:"type"`
	RecipientEmail string `json:"recipient_email"`
	AttemptNumber  int    `json:"attempt_number"`
	ErrorMessage   string `json:"error_message"`
	FinalFailure   bool   `json:"final_failure"`
}
