package domain

import (
	"encoding/json"
	"time"
)

const (
	MessageStatusQueued     = "queued"
	MessageStatusProcessing = "processing"
	MessageStatusAccepted   = "accepted"
	MessageStatusDelivered  = "delivered"
	MessageStatusDelayed    = "delayed"
	MessageStatusBounced    = "bounced"
	MessageStatusComplained = "complained"
	MessageStatusFailed     = "failed"
	MessageStatusCancelled  = "cancelled"
	MessageStatusDLQ        = "dlq"
)

const (
	MessageSourceCampaign        = "campaign"
	MessageSourceTransactional   = "transactional"
)

const (
	MessageTypeMarketing     = "marketing"
	MessageTypeTransactional = "transactional"
)

const (
	AttemptStatusStarted         = "started"
	AttemptStatusAccepted        = "accepted"
	AttemptStatusTemporaryFailed = "temporary_failed"
	AttemptStatusPermanentFailed = "permanent_failed"
)

const (
	RetryStatusPending   = "pending"
	RetryStatusScheduled = "scheduled"
	RetryStatusExhausted = "exhausted"
	RetryStatusCancelled = "cancelled"
)

const (
	TxRequestStatusAccepted   = "accepted"
	TxRequestStatusProcessing = "processing"
	TxRequestStatusCompleted  = "completed"
	TxRequestStatusFailed     = "failed"
)

type RecipientSnapshot struct {
	ContactID       string         `json:"contact_id"`
	Email           string         `json:"email"`
	EmailNormalized string         `json:"email_normalized"`
	FirstName       string         `json:"first_name"`
	LastName        string         `json:"last_name"`
	Tags            []string       `json:"tags"`
	Attributes      map[string]any `json:"attributes"`
}

type Message struct {
	ID                      string
	WorkspaceID             string
	CampaignID              string
	CampaignCandidateID     string
	TransactionalRequestID  string
	ContactID               string
	RecipientEmailNormalized string
	RecipientSnapshot       RecipientSnapshot
	TemplateID              string
	TemplateVersionID       string
	SenderDomainID          string
	MessageType             string
	SourceType              string
	Status                  string
	ScheduledAt             *time.Time
	QueuedAt                *time.Time
	ProcessingStartedAt     *time.Time
	AcceptedAt              *time.Time
	DeliveredAt             *time.Time
	BouncedAt               *time.Time
	ComplainedAt            *time.Time
	FailedAt                *time.Time
	LastErrorClass          string
	LastErrorMessage        string
	Provider                string
	ProviderMessageID       string
	CreatedAt               time.Time
	UpdatedAt               time.Time
}

type DeliveryAttempt struct {
	ID               string
	WorkspaceID      string
	MessageID        string
	AttemptNo        int
	Provider         string
	Status           string
	RequestSnapshot  json.RawMessage
	ResponseSnapshot json.RawMessage
	ErrorClass       string
	ErrorMessage     string
	StartedAt        time.Time
	FinishedAt       *time.Time
}

type RetryState struct {
	ID               string
	WorkspaceID      string
	MessageID        string
	RetryCount       int
	MaxRetries       int
	NextAttemptAt    *time.Time
	LastErrorClass   string
	LastErrorMessage string
	Status           string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type TransactionalSendRequest struct {
	ID              string
	WorkspaceID     string
	IdempotencyKey  string
	Status          string
	RequestPayload  json.RawMessage
	CreatedAt       time.Time
	UpdatedAt       time.Time
	CompletedAt     *time.Time
	FailedAt        *time.Time
}

func ValidMessageStatus(s string) bool {
	switch s {
	case MessageStatusQueued, MessageStatusProcessing, MessageStatusAccepted,
		MessageStatusDelivered, MessageStatusDelayed, MessageStatusBounced,
		MessageStatusComplained, MessageStatusFailed, MessageStatusCancelled,
		MessageStatusDLQ:
		return true
	default:
		return false
	}
}

func ValidMessageSourceType(s string) bool {
	switch s {
	case MessageSourceCampaign, MessageSourceTransactional:
		return true
	default:
		return false
	}
}

func ValidMessageType(s string) bool {
	switch s {
	case MessageTypeMarketing, MessageTypeTransactional:
		return true
	default:
		return false
	}
}

func ValidAttemptStatus(s string) bool {
	switch s {
	case AttemptStatusStarted, AttemptStatusAccepted,
		AttemptStatusTemporaryFailed, AttemptStatusPermanentFailed:
		return true
	default:
		return false
	}
}

func ValidRetryStatus(s string) bool {
	switch s {
	case RetryStatusPending, RetryStatusScheduled,
		RetryStatusExhausted, RetryStatusCancelled:
		return true
	default:
		return false
	}
}

func ValidateRecipientSnapshot(rs RecipientSnapshot) error {
	if rs.EmailNormalized == "" {
		return ErrPayloadInvalid
	}
	return nil
}

func ValidateQueuedMessage(msg Message) error {
	if msg.WorkspaceID == "" {
		return ErrPayloadInvalid
	}
	if msg.RecipientEmailNormalized == "" {
		return ErrPayloadInvalid
	}
	if err := ValidateRecipientSnapshot(msg.RecipientSnapshot); err != nil {
		return err
	}
	if msg.TemplateID == "" {
		return ErrPayloadInvalid
	}
	if msg.SenderDomainID == "" {
		return ErrPayloadInvalid
	}
	if !ValidMessageType(msg.MessageType) {
		return ErrPayloadInvalid
	}
	if !ValidMessageSourceType(msg.SourceType) {
		return ErrPayloadInvalid
	}
	return nil
}

func (m Message) CanStartProcessing() bool {
	return m.Status == MessageStatusQueued
}

func (m Message) CanMarkAccepted() bool {
	return m.Status == MessageStatusProcessing
}

func (m Message) CanMarkDelivered() bool {
	return m.Status == MessageStatusAccepted || m.Status == MessageStatusDelayed
}

func (m Message) CanMarkBounced() bool {
	return m.Status == MessageStatusQueued || m.Status == MessageStatusProcessing ||
		m.Status == MessageStatusAccepted || m.Status == MessageStatusDelayed
}

func (m Message) CanMarkComplained() bool {
	return m.Status == MessageStatusAccepted || m.Status == MessageStatusDelivered ||
		m.Status == MessageStatusDelayed
}

func (m Message) CanFail() bool {
	return !m.IsTerminal()
}

func (m Message) IsTerminal() bool {
	switch m.Status {
	case MessageStatusDelivered, MessageStatusBounced, MessageStatusComplained,
		MessageStatusFailed, MessageStatusCancelled, MessageStatusDLQ:
		return true
	default:
		return false
	}
}
