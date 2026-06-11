package domain

import (
	"strings"
	"time"
)

const (
	EventTypeQueued         = "queued"
	EventTypeAccepted       = "accepted"
	EventTypeDelivered      = "delivered"
	EventTypeBounced        = "bounced"
	EventTypeComplained     = "complained"
	EventTypeRetryScheduled = "retry_scheduled"
	EventTypeOpened         = "opened"
	EventTypeClicked        = "clicked"
	EventTypeUnsubscribed   = "unsubscribed"
	EventTypeSuppressed     = "suppressed"
)

var KnownEventTypes = map[string]bool{
	EventTypeQueued:         true,
	EventTypeAccepted:       true,
	EventTypeDelivered:      true,
	EventTypeBounced:        true,
	EventTypeComplained:     true,
	EventTypeRetryScheduled: true,
	EventTypeOpened:         true,
	EventTypeClicked:        true,
	EventTypeUnsubscribed:   true,
	EventTypeSuppressed:     true,
}

type EmailEventFact struct {
	ID                string
	SourceEventID     string
	SourceEventType   string
	WorkspaceID       string
	CampaignID        string
	MessageID         string
	Provider          string
	ProviderMessageID string
	ProviderEventID   string
	EventType         string
	RecipientDomain   string
	OccurredAt        time.Time
	ReceivedAt        time.Time
	Metadata          map[string]any
	CreatedAt         time.Time
}

type CampaignDeliverySummary struct {
	WorkspaceID         string
	CampaignID          string
	QueuedCount         int64
	AcceptedCount       int64
	DeliveredCount      int64
	BouncedCount        int64
	ComplainedCount     int64
	OpenedCount         int64
	ClickedCount        int64
	UnsubscribedCount   int64
	RetryScheduledCount int64
	LastEventAt         *time.Time
	LastUpdatedAt       time.Time
}

type WorkspaceAnalyticsOverview struct {
	WorkspaceID         string
	QueuedCount         int64
	AcceptedCount       int64
	DeliveredCount      int64
	BouncedCount        int64
	ComplainedCount     int64
	OpenedCount         int64
	ClickedCount        int64
	UnsubscribedCount   int64
	RetryScheduledCount int64
	LastEventAt         *time.Time
	LastUpdatedAt       time.Time
}

type DeliverabilityProjection struct {
	WorkspaceID     string
	Provider        string
	RecipientDomain string
	DeliveredCount  int64
	BouncedCount    int64
	ComplainedCount int64
	OpenedCount     int64
	ClickedCount    int64
	LastEventAt     *time.Time
	LastUpdatedAt   time.Time
}

type DashboardOverview struct {
	Status              string
	WorkspaceID         string
	QueuedCount         int64
	AcceptedCount       int64
	DeliveredCount      int64
	BouncedCount        int64
	ComplainedCount     int64
	OpenedCount         int64
	ClickedCount        int64
	UnsubscribedCount   int64
	RetryScheduledCount int64
	LastEventAt         *time.Time
	LastUpdatedAt       time.Time
}

type CampaignAnalytics struct {
	Status              string
	WorkspaceID         string
	CampaignID          string
	QueuedCount         int64
	AcceptedCount       int64
	DeliveredCount      int64
	BouncedCount        int64
	ComplainedCount     int64
	OpenedCount         int64
	ClickedCount        int64
	UnsubscribedCount   int64
	RetryScheduledCount int64
	DeliveryRate        float64
	BounceRate          float64
	ComplaintRate       float64
	OpenRate            float64
	ClickRate           float64
	UnsubscribeRate     float64
	LastEventAt         *time.Time
	LastUpdatedAt       time.Time
}

type DeliverabilityRow struct {
	Provider        string
	RecipientDomain string
	DeliveredCount  int64
	BouncedCount    int64
	ComplainedCount int64
	OpenedCount     int64
	ClickedCount    int64
	BounceRate      float64
	ComplaintRate   float64
	LastEventAt     *time.Time
	LastUpdatedAt   time.Time
}

type DeliverabilityResult struct {
	Status string
	Items  []DeliverabilityRow
}

func ValidateFact(workspaceID, sourceEventID, eventType string, occurredAt, receivedAt time.Time) error {
	if strings.TrimSpace(workspaceID) == "" {
		return ErrAnalyticsEventInvalid
	}
	if strings.TrimSpace(sourceEventID) == "" {
		return ErrAnalyticsEventInvalid
	}
	if !KnownEventTypes[eventType] {
		return ErrAnalyticsEventInvalid
	}
	if occurredAt.IsZero() {
		return ErrAnalyticsEventInvalid
	}
	if receivedAt.IsZero() {
		return ErrAnalyticsEventInvalid
	}
	return nil
}

func NormalizeString(s string) string {
	return strings.TrimSpace(strings.ToLower(s))
}

type DeliverabilityFilter struct {
	Provider        string
	RecipientDomain string
}

func ComputeRate(numerator, denominator int64) float64 {
	if denominator == 0 {
		return 0
	}
	return float64(numerator) / float64(denominator) * 100
}
