package domain

import (
	"strings"
	"time"
)

type NotificationType string

const (
	NotificationTypeWelcomeEmail    NotificationType = "welcome_email"
	NotificationTypeInvitationEmail NotificationType = "invitation_email"
	NotificationTypeSystemAlert     NotificationType = "system_alert"
)

func ValidNotificationType(t NotificationType) bool {
	switch t {
	case NotificationTypeWelcomeEmail, NotificationTypeInvitationEmail, NotificationTypeSystemAlert:
		return true
	default:
		return false
	}
}

type NotificationStatus string

const (
	NotificationStatusPending  NotificationStatus = "pending"
	NotificationStatusQueued   NotificationStatus = "queued"
	NotificationStatusSending  NotificationStatus = "sending"
	NotificationStatusSent     NotificationStatus = "sent"
	NotificationStatusFailed   NotificationStatus = "failed"
	NotificationStatusRetrying NotificationStatus = "retrying"
)

func ValidNotificationStatus(s NotificationStatus) bool {
	switch s {
	case NotificationStatusPending, NotificationStatusQueued, NotificationStatusSending,
		NotificationStatusSent, NotificationStatusFailed, NotificationStatusRetrying:
		return true
	default:
		return false
	}
}

func CanTransitionTo(current, next NotificationStatus) bool {
	switch current {
	case NotificationStatusPending:
		return next == NotificationStatusQueued || next == NotificationStatusSending
	case NotificationStatusQueued:
		return next == NotificationStatusSending || next == NotificationStatusPending
	case NotificationStatusSending:
		return next == NotificationStatusSent || next == NotificationStatusFailed || next == NotificationStatusRetrying
	case NotificationStatusRetrying:
		return next == NotificationStatusSending || next == NotificationStatusFailed
	case NotificationStatusSent, NotificationStatusFailed:
		return false
	default:
		return false
	}
}

type AttemptStatus string

const (
	AttemptStatusSending AttemptStatus = "sending"
	AttemptStatusSent    AttemptStatus = "sent"
	AttemptStatusFailed  AttemptStatus = "failed"
)

func ValidAttemptStatus(s AttemptStatus) bool {
	switch s {
	case AttemptStatusSending, AttemptStatusSent, AttemptStatusFailed:
		return true
	default:
		return false
	}
}

type NotificationMessage struct {
	ID              string
	WorkspaceID     *string
	Type            NotificationType
	Status          NotificationStatus
	RecipientEmail  string
	RecipientUserID *string
	Subject         string
	BodyText        string
	BodyHTML        *string
	MaxAttempts     int
	AttemptCount    int
	LastAttemptAt   *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type NotificationAttempt struct {
	ID                    string
	NotificationMessageID string
	AttemptNumber         int
	Status                AttemptStatus
	Provider              string
	ProviderMessageID     *string
	ErrorMessage          *string
	AttemptedAt           time.Time
}

type NotificationFilter struct {
	WorkspaceID     *string
	RecipientUserID string
	Type            string
	Status          string
	From            *time.Time
	To              *time.Time
	Cursor          string
	Limit           int
}

type SendWelcomeEmailInput struct {
	UserID          string
	Email           string
	FrontendBaseURL string
}

type SendInvitationEmailInput struct {
	WorkspaceID     string
	InvitedByEmail  string
	InvitedByUserID string
	InviteeEmail    string
	Role            string
	FrontendBaseURL string
}

type SendSystemAlertInput struct {
	RecipientEmail string
	Subject        string
	Body           string
	WorkspaceID    string
}

func ValidateRecipientEmail(email string) error {
	email = strings.TrimSpace(email)
	if email == "" {
		return ErrRecipientEmailInvalid
	}
	if !strings.Contains(email, "@") {
		return ErrRecipientEmailInvalid
	}
	if !strings.Contains(email, ".") {
		return ErrRecipientEmailInvalid
	}
	return nil
}

func ValidateSubject(subject string) error {
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return ErrSubjectInvalid
	}
	if len([]rune(subject)) > 500 {
		return ErrSubjectInvalid
	}
	return nil
}
