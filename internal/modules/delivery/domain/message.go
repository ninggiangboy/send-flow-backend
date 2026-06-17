package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
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
	MessageSourceCampaign      = "campaign"
	MessageSourceTransactional = "transactional"
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

const (
	MessageModeTemplate = "template"
	MessageModeRaw      = "raw"
)

const (
	RecipientRoleTo  = "to"
	RecipientRoleCC  = "cc"
	RecipientRoleBCC = "bcc"
)

const (
	MessageEventRequestAccepted   = "request_accepted"
	MessageEventQueued            = "queued"
	MessageEventProcessingStarted = "processing_started"
	MessageEventProviderAccepted  = "provider_accepted"
	MessageEventDelivered         = "delivered"
	MessageEventBounced           = "bounced"
	MessageEventComplained        = "complained"
	MessageEventRetryScheduled    = "retry_scheduled"
	MessageEventFailed            = "failed"
	MessageEventSuppressed        = "suppressed"
	MessageEventRequestCompleted  = "request_completed"
)

const (
	AttachmentDispositionAttachment = "attachment"
	AttachmentDispositionInline     = "inline"
)

func ClassifyProviderEvent(providerEventType string) (string, bool) {
	switch providerEventType {
	case "delivered":
		return MessageStatusDelivered, true
	case "bounced":
		return MessageStatusBounced, true
	case "complained":
		return MessageStatusComplained, true
	case "delayed":
		return MessageStatusDelayed, true
	case "rejected":
		return MessageStatusFailed, true
	case "accepted", "opened", "clicked", "unsubscribed", "rendering_failed":
		return "", false
	default:
		return "", false
	}
}

func CanTransitionToStatus(currentStatus, newStatus string) bool {
	if currentStatus == newStatus {
		return true
	}
	switch currentStatus {
	case MessageStatusBounced, MessageStatusFailed,
		MessageStatusCancelled, MessageStatusDLQ:
		return false
	case MessageStatusComplained:
		return newStatus == MessageStatusComplained
	case MessageStatusDelivered:
		return newStatus == MessageStatusDelivered || newStatus == MessageStatusComplained
	case MessageStatusDelayed:
		return newStatus != MessageStatusAccepted
	case MessageStatusAccepted:
		return true
	case MessageStatusProcessing:
		return newStatus != MessageStatusAccepted
	case MessageStatusQueued:
		return newStatus != MessageStatusDelivered && newStatus != MessageStatusComplained
	default:
		return false
	}
}

type RecipientSnapshot struct {
	ContactID       string         `json:"contact_id"`
	Email           string         `json:"email"`
	EmailNormalized string         `json:"email_normalized"`
	FirstName       string         `json:"first_name"`
	LastName        string         `json:"last_name"`
	Tags            []string       `json:"tags"`
	Attributes      map[string]any `json:"attributes"`
	TemplateData    map[string]any `json:"template_data,omitempty"`
}

type RecipientTarget struct {
	Email string `json:"email"`
	Name  string `json:"name,omitempty"`
}

type FilePart struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
	SHA256      string `json:"sha256"`
	Disposition string `json:"disposition,omitempty"`
	ContentID   string `json:"content_id,omitempty"`
}

type CanonicalSendRequest struct {
	Mode              string            `json:"mode"`
	WorkspaceID       string            `json:"workspace_id"`
	APIKeyID          string            `json:"api_key_id"`
	IdempotencyKey    string            `json:"idempotency_key,omitempty"`
	SenderDomainID    string            `json:"sender_domain_id"`
	SenderName        string            `json:"sender_name,omitempty"`
	Subject           string            `json:"subject,omitempty"`
	TemplateID        string            `json:"template_id,omitempty"`
	TemplateVersionID string            `json:"template_version_id,omitempty"`
	TemplateData      map[string]any    `json:"template_data,omitempty"`
	TextBody          string            `json:"text_body,omitempty"`
	HTMLBody          string            `json:"html_body,omitempty"`
	ReplyTo           string            `json:"reply_to,omitempty"`
	To                []RecipientTarget `json:"to"`
	CC                []RecipientTarget `json:"cc,omitempty"`
	BCC               []RecipientTarget `json:"bcc,omitempty"`
	Metadata          map[string]any    `json:"metadata,omitempty"`
	Tags              []string          `json:"tags,omitempty"`
	Headers           map[string]string `json:"headers,omitempty"`
	Attachments       []FilePart        `json:"attachments,omitempty"`
	Now               time.Time         `json:"-"`
}

// ComputeHash returns a SHA-256 hex digest of the canonical request
// including attachment digests. Used for idempotency comparison.
func (c *CanonicalSendRequest) ComputeHash() (string, error) {
	h := sha256.New()
	enc := json.NewEncoder(h)
	// Encode everything except Now and Attachment data
	if err := enc.Encode(c); err != nil {
		return "", err
	}
	// Include attachment SHA256 digests in order
	for _, att := range c.Attachments {
		h.Write([]byte(att.SHA256))
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

type MessageEvent struct {
	ID                     string          `json:"id"`
	WorkspaceID            string          `json:"workspace_id"`
	MessageID              string          `json:"message_id"`
	TransactionalRequestID string          `json:"transactional_request_id,omitempty"`
	EventType              string          `json:"event_type"`
	Status                 string          `json:"status"`
	ReasonCode             string          `json:"reason_code,omitempty"`
	ReasonMessage          string          `json:"reason_message,omitempty"`
	Metadata               json.RawMessage `json:"metadata,omitempty"`
	OccurredAt             time.Time       `json:"occurred_at"`
	CreatedAt              time.Time       `json:"created_at"`
}

type AttachmentManifest struct {
	ID                     string    `json:"id"`
	WorkspaceID            string    `json:"workspace_id"`
	TransactionalRequestID string    `json:"transactional_request_id"`
	StorageKey             string    `json:"storage_key"`
	OriginalFilename       string    `json:"original_filename"`
	ContentType            string    `json:"content_type"`
	ByteSize               int64     `json:"byte_size"`
	SHA256Digest           string    `json:"sha256_digest"`
	Disposition            string    `json:"disposition"`
	ContentID              string    `json:"content_id,omitempty"`
	CreatedAt              time.Time `json:"created_at"`
}

type Message struct {
	ID                       string
	WorkspaceID              string
	CampaignID               string
	CampaignCandidateID      string
	TransactionalRequestID   string
	ContactID                string
	RecipientEmailNormalized string
	RecipientSnapshot        RecipientSnapshot
	TemplateID               string
	TemplateVersionID        string
	SenderDomainID           string
	Subject                  string
	SenderName               string
	TextBody                 string
	HTMLBody                 string
	ReplyTo                  string
	Headers                  map[string]string
	RecipientRole            string
	SourceAPIKeyID           string
	MessageType              string
	SourceType               string
	Status                   string
	ScheduledAt              *time.Time
	QueuedAt                 *time.Time
	ProcessingStartedAt      *time.Time
	AcceptedAt               *time.Time
	DeliveredAt              *time.Time
	BouncedAt                *time.Time
	ComplainedAt             *time.Time
	FailedAt                 *time.Time
	LastErrorClass           string
	LastErrorMessage         string
	Provider                 string
	ProviderMessageID        string
	CreatedAt                time.Time
	UpdatedAt                time.Time
}

func NewMessage(id, workspaceID, recipientEmailNormalized, messageType, sourceType string, now time.Time) (*Message, error) {
	if id == "" {
		return nil, errors.New("message id is required")
	}
	if workspaceID == "" {
		return nil, errors.New("workspace id is required")
	}
	if recipientEmailNormalized == "" {
		return nil, errors.New("recipient email is required")
	}
	if !ValidMessageType(messageType) {
		return nil, errors.New("valid message type is required")
	}
	if !ValidMessageSourceType(sourceType) {
		return nil, errors.New("valid source type is required")
	}
	return &Message{
		ID:                       id,
		WorkspaceID:              workspaceID,
		RecipientEmailNormalized: recipientEmailNormalized,
		MessageType:              messageType,
		SourceType:               sourceType,
		Status:                   MessageStatusQueued,
		CreatedAt:                now,
		UpdatedAt:                now,
	}, nil
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
	IdempotencyKey  *string
	Status          string
	RequestPayload  json.RawMessage
	Mode            string
	Subject         string
	SenderName      string
	SourceAPIKeyID  string
	RequestHash     string
	TotalRecipients int
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

func ValidMessageMode(s string) bool {
	return s == MessageModeTemplate || s == MessageModeRaw
}

func ValidRecipientRole(s string) bool {
	return s == RecipientRoleTo || s == RecipientRoleCC || s == RecipientRoleBCC
}

func ValidMessageEventType(s string) bool {
	switch s {
	case MessageEventRequestAccepted, MessageEventQueued, MessageEventProcessingStarted,
		MessageEventProviderAccepted, MessageEventDelivered, MessageEventBounced,
		MessageEventComplained, MessageEventRetryScheduled, MessageEventFailed,
		MessageEventSuppressed, MessageEventRequestCompleted:
		return true
	default:
		return false
	}
}

func ValidAttachmentDisposition(s string) bool {
	return s == AttachmentDispositionAttachment || s == AttachmentDispositionInline
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
	if msg.TemplateID == "" && msg.MessageType == MessageTypeTransactional && msg.SourceType == MessageSourceTransactional {
		// TemplateID can be empty for raw mode transactional sends
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
	if msg.RecipientRole != "" && !ValidRecipientRole(msg.RecipientRole) {
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
