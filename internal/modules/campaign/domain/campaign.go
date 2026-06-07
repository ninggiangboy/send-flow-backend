package domain

import (
	"strings"
	"time"
)

type CampaignStatus string

const (
	CampaignStatusDraft     CampaignStatus = "draft"
	CampaignStatusScheduled CampaignStatus = "scheduled"
	CampaignStatusRunning   CampaignStatus = "running"
	CampaignStatusPaused    CampaignStatus = "paused"
	CampaignStatusCancelled CampaignStatus = "cancelled"
	CampaignStatusCompleted CampaignStatus = "completed"
)

type AudienceType string

const (
	AudienceTypeContacts AudienceType = "contacts"
	AudienceTypeList     AudienceType = "list"
	AudienceTypeSegment  AudienceType = "segment"
)

type MessageType string

const (
	MessageTypeMarketing     MessageType = "marketing"
	MessageTypeTransactional MessageType = "transactional"
)

type CandidateStatus string

const (
	CandidateStatusPlanned CandidateStatus = "planned"
	CandidateStatusQueued  CandidateStatus = "queued"
	CandidateStatusSkipped CandidateStatus = "skipped"
)

type Campaign struct {
	ID                string
	WorkspaceID       string
	Name              string
	Status            CampaignStatus
	AudienceRef       AudienceRef
	TemplateRef       TemplateRef
	SenderDomainID    string
	MessageType       MessageType
	ScheduledAt       *time.Time
	PlannedRecipients int64
	CreatedAt         time.Time
	UpdatedAt         time.Time
	CancelledAt       *time.Time
	PausedAt          *time.Time
	CompletedAt       *time.Time
}

type AudienceRef struct {
	Type       AudienceType
	ID         string
	ContactIDs []string
}

type TemplateRef struct {
	TemplateID        string
	TemplateVersionID string
}

type CampaignMessageCandidate struct {
	ID                string
	WorkspaceID       string
	CampaignID        string
	ContactID         string
	EmailNormalized   string
	RecipientSnapshot RecipientSnapshot
	Status            CandidateStatus
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type RecipientSnapshot struct {
	ContactID       string         `json:"contact_id"`
	Email           string         `json:"email"`
	EmailNormalized string         `json:"email_normalized"`
	FirstName       string         `json:"first_name"`
	LastName        string         `json:"last_name"`
	Tags            []string       `json:"tags"`
	Attributes      map[string]any `json:"attributes"`
}

func ValidCampaignStatus(s string) bool {
	switch CampaignStatus(s) {
	case CampaignStatusDraft, CampaignStatusScheduled, CampaignStatusRunning,
		CampaignStatusPaused, CampaignStatusCancelled, CampaignStatusCompleted:
		return true
	default:
		return false
	}
}

func ValidAudienceType(s string) bool {
	switch AudienceType(s) {
	case AudienceTypeContacts, AudienceTypeList, AudienceTypeSegment:
		return true
	default:
		return false
	}
}

func ValidMessageType(s string) bool {
	switch MessageType(s) {
	case MessageTypeMarketing, MessageTypeTransactional:
		return true
	default:
		return false
	}
}

func ValidCandidateStatus(s string) bool {
	switch CandidateStatus(s) {
	case CandidateStatusPlanned, CandidateStatusQueued, CandidateStatusSkipped:
		return true
	default:
		return false
	}
}

func ValidateCampaignName(name string) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return ErrPayloadInvalid
	}
	return nil
}

func ValidateAudienceRef(ref AudienceRef) error {
	switch ref.Type {
	case AudienceTypeContacts:
		if len(ref.ContactIDs) == 0 {
			return ErrAudienceNotReady
		}
	case AudienceTypeList:
		if ref.ID == "" {
			return ErrPayloadInvalid
		}
	case AudienceTypeSegment:
		if ref.ID == "" {
			return ErrPayloadInvalid
		}
	default:
		return ErrPayloadInvalid
	}
	return nil
}

func (c Campaign) CanUpdateDraft() bool {
	return c.Status == CampaignStatusDraft
}

func (c Campaign) CanSchedule() bool {
	return c.Status == CampaignStatusDraft
}

func (c Campaign) CanPause() bool {
	return c.Status == CampaignStatusScheduled || c.Status == CampaignStatusRunning
}

func (c Campaign) CanResume() bool {
	return c.Status == CampaignStatusPaused
}

func (c Campaign) CanCancel() bool {
	return c.Status == CampaignStatusDraft || c.Status == CampaignStatusScheduled || c.Status == CampaignStatusPaused
}
