package domain

import (
	"fmt"
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

var SupportedIntervals = map[string]bool{
	"hour": true,
	"day":  true,
	"week": true,
}

var SupportedGroups = map[string]bool{
	"provider":         true,
	"recipient_domain": true,
	"event_type":       true,
}

func ValidateInterval(v string) error {
	if !SupportedIntervals[v] {
		return fmt.Errorf("%w: unsupported interval %q, must be hour, day, or week", ErrAnalyticsQueryInvalid, v)
	}
	return nil
}

func ValidateGroupBy(v string) error {
	if !SupportedGroups[v] {
		return fmt.Errorf("%w: unsupported group %q, must be provider, recipient_domain, or event_type", ErrAnalyticsQueryInvalid, v)
	}
	return nil
}

func ValidateTimeRange(from, to time.Time) error {
	if !from.IsZero() && !to.IsZero() && !from.Before(to) {
		return fmt.Errorf("%w: from must be before to", ErrAnalyticsQueryInvalid)
	}
	return nil
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

type CampaignQueryFilter struct {
	WorkspaceID string
	CampaignID  string
	From        time.Time
	To          time.Time
	EventType   string
	Provider    string
	Domain      string
	Limit       int
	Cursor      string
}

func (f CampaignQueryFilter) Validate() error {
	if strings.TrimSpace(f.WorkspaceID) == "" {
		return fmt.Errorf("%w: workspace_id is required", ErrAnalyticsQueryInvalid)
	}
	if strings.TrimSpace(f.CampaignID) == "" {
		return fmt.Errorf("%w: campaign_id is required", ErrAnalyticsQueryInvalid)
	}
	if err := ValidateTimeRange(f.From, f.To); err != nil {
		return err
	}
	if f.EventType != "" && !KnownEventTypes[f.EventType] {
		return fmt.Errorf("%w: unsupported event_type %q", ErrAnalyticsQueryInvalid, f.EventType)
	}
	if f.Limit <= 0 || f.Limit > 100 {
		return fmt.Errorf("%w: limit must be between 1 and 100", ErrAnalyticsQueryInvalid)
	}
	return nil
}

type CampaignFunnel struct {
	Status              string  `json:"status"`
	WorkspaceID         string  `json:"workspace_id"`
	CampaignID          string  `json:"campaign_id"`
	QueuedCount         int64   `json:"queued_count"`
	AcceptedCount       int64   `json:"accepted_count"`
	DeliveredCount      int64   `json:"delivered_count"`
	BouncedCount        int64   `json:"bounced_count"`
	ComplainedCount     int64   `json:"complained_count"`
	OpenedCount         int64   `json:"opened_count"`
	ClickedCount        int64   `json:"clicked_count"`
	UnsubscribedCount   int64   `json:"unsubscribed_count"`
	RetryScheduledCount int64   `json:"retry_scheduled_count"`
	DeliveryRate        float64 `json:"delivery_rate"`
	BounceRate          float64 `json:"bounce_rate"`
	ComplaintRate       float64 `json:"complaint_rate"`
	OpenRate            float64 `json:"open_rate"`
	ClickRate           float64 `json:"click_rate"`
	UnsubscribeRate     float64 `json:"unsubscribe_rate"`
	LastEventAt         string  `json:"last_event_at,omitempty"`
}

type CampaignTimeSeriesBucket struct {
	BucketStart time.Time `json:"bucket_start"`
	EventType   string    `json:"event_type"`
	Count       int64     `json:"count"`
}

type CampaignTimeSeriesResult struct {
	Status      string                     `json:"status"`
	WorkspaceID string                     `json:"workspace_id"`
	CampaignID  string                     `json:"campaign_id"`
	Buckets     []CampaignTimeSeriesBucket `json:"buckets"`
}

type CampaignBreakdownRow struct {
	GroupKey    string  `json:"group_key"`
	EventType   string  `json:"event_type"`
	Count       int64   `json:"count"`
	Rate        float64 `json:"rate"`
	LastEventAt string  `json:"last_event_at,omitempty"`
}

type CampaignBreakdownResult struct {
	Status      string                 `json:"status"`
	WorkspaceID string                 `json:"workspace_id"`
	CampaignID  string                 `json:"campaign_id"`
	GroupBy     string                 `json:"group_by"`
	Rows        []CampaignBreakdownRow `json:"rows"`
}

type CampaignEventRow struct {
	SourceEventID     string `json:"source_event_id"`
	SourceEventType   string `json:"source_event_type"`
	MessageID         string `json:"message_id,omitempty"`
	Provider          string `json:"provider,omitempty"`
	ProviderMessageID string `json:"provider_message_id,omitempty"`
	EventType         string `json:"event_type"`
	RecipientDomain   string `json:"recipient_domain,omitempty"`
	OccurredAt        string `json:"occurred_at"`
	ReceivedAt        string `json:"received_at"`
}

type CampaignEventsResult struct {
	Status      string             `json:"status"`
	WorkspaceID string             `json:"workspace_id"`
	CampaignID  string             `json:"campaign_id"`
	Events      []CampaignEventRow `json:"events"`
	NextCursor  string             `json:"next_cursor,omitempty"`
}

type DeliverabilityQueryFilter struct {
	WorkspaceID string
	From        time.Time
	To          time.Time
	Provider    string
	Domain      string
	CampaignID  string
	GroupBy     string
	Limit       int
	Cursor      string
}

func (f DeliverabilityQueryFilter) Validate() error {
	if strings.TrimSpace(f.WorkspaceID) == "" {
		return fmt.Errorf("%w: workspace_id is required", ErrAnalyticsQueryInvalid)
	}
	if err := ValidateTimeRange(f.From, f.To); err != nil {
		return err
	}
	if f.Limit <= 0 || f.Limit > 100 {
		return fmt.Errorf("%w: limit must be between 1 and 100", ErrAnalyticsQueryInvalid)
	}
	return nil
}

type DeliverabilityTimeSeriesBucket struct {
	BucketStart     time.Time `json:"bucket_start"`
	Provider        string    `json:"provider,omitempty"`
	RecipientDomain string    `json:"recipient_domain,omitempty"`
	EventType       string    `json:"event_type"`
	Count           int64     `json:"count"`
}

type DeliverabilityTimeSeriesResult struct {
	Status      string                           `json:"status"`
	WorkspaceID string                           `json:"workspace_id"`
	Buckets     []DeliverabilityTimeSeriesBucket `json:"buckets"`
}

type DeliverabilityBreakdownRow struct {
	GroupKey        string  `json:"-"`
	Provider        string  `json:"provider,omitempty"`
	RecipientDomain string  `json:"recipient_domain,omitempty"`
	EventType       string  `json:"event_type"`
	Count           int64   `json:"count"`
	Rate            float64 `json:"rate"`
	LastEventAt     string  `json:"last_event_at,omitempty"`
}

type DeliverabilityBreakdownResult struct {
	Status      string                       `json:"status"`
	WorkspaceID string                       `json:"workspace_id"`
	GroupBy     string                       `json:"group_by"`
	Rows        []DeliverabilityBreakdownRow `json:"rows"`
}

type DeliverabilityLatencyRow struct {
	Provider        string  `json:"provider"`
	RecipientDomain string  `json:"recipient_domain,omitempty"`
	EventType       string  `json:"event_type"`
	Count           int64   `json:"count"`
	P50LatencyMs    float64 `json:"p50_latency_ms"`
	P95LatencyMs    float64 `json:"p95_latency_ms"`
	P99LatencyMs    float64 `json:"p99_latency_ms"`
	AvgLatencyMs    float64 `json:"avg_latency_ms"`
}

type DeliverabilityLatencyResult struct {
	Status      string                     `json:"status"`
	WorkspaceID string                     `json:"workspace_id"`
	Rows        []DeliverabilityLatencyRow `json:"rows"`
}

type DeliverabilityIncidentRow struct {
	Provider        string  `json:"provider"`
	RecipientDomain string  `json:"recipient_domain,omitempty"`
	EventType       string  `json:"event_type"`
	IncidentStart   string  `json:"incident_start"`
	IncidentEnd     string  `json:"incident_end,omitempty"`
	EventCount      int64   `json:"event_count"`
	Rate            float64 `json:"rate"`
}

type DeliverabilityIncidentResult struct {
	Status      string                      `json:"status"`
	WorkspaceID string                      `json:"workspace_id"`
	Provider    string                      `json:"provider,omitempty"`
	Domain      string                      `json:"domain,omitempty"`
	Rows        []DeliverabilityIncidentRow `json:"rows"`
}

// ── Phase 6: Product Usage, Risk, Forecasting, and Anomalies ──

type UsageQueryFilter struct {
	WorkspaceID string
	From        time.Time
	To          time.Time
	Interval    string
	Limit       int
	Cursor      string
}

func (f UsageQueryFilter) Validate() error {
	if strings.TrimSpace(f.WorkspaceID) == "" {
		return fmt.Errorf("%w: workspace_id is required", ErrAnalyticsQueryInvalid)
	}
	if err := ValidateTimeRange(f.From, f.To); err != nil {
		return err
	}
	if f.Interval != "" && !SupportedIntervals[f.Interval] {
		return fmt.Errorf("%w: unsupported interval %q", ErrAnalyticsQueryInvalid, f.Interval)
	}
	if f.Limit <= 0 || f.Limit > 100 {
		return fmt.Errorf("%w: limit must be between 1 and 100", ErrAnalyticsQueryInvalid)
	}
	return nil
}

type UsageTimeSeriesBucket struct {
	BucketStart time.Time `json:"bucket_start"`
	EventType   string    `json:"event_type"`
	Count       int64     `json:"count"`
}

type UsageTimeSeriesResult struct {
	Status      string                  `json:"status"`
	WorkspaceID string                  `json:"workspace_id"`
	Buckets     []UsageTimeSeriesBucket `json:"buckets"`
}

type UsageFeatureRow struct {
	Feature     string `json:"feature"`
	ActiveCount int64  `json:"active_count"`
	EventCount  int64  `json:"event_count"`
	BucketStart string `json:"bucket_start"`
}

type UsageFeaturesResult struct {
	Status      string            `json:"status"`
	WorkspaceID string            `json:"workspace_id"`
	Rows        []UsageFeatureRow `json:"rows"`
}

type RiskSignalRow struct {
	SignalType string  `json:"signal_type"`
	Severity   string  `json:"severity"`
	Metric     string  `json:"metric"`
	Value      float64 `json:"value"`
	Threshold  float64 `json:"threshold"`
	DetectedAt string  `json:"detected_at"`
	CampaignID string  `json:"campaign_id,omitempty"`
}

type RiskSignalsResult struct {
	Status      string          `json:"status"`
	WorkspaceID string          `json:"workspace_id"`
	Signals     []RiskSignalRow `json:"signals"`
}

type SendVolumeForecastRow struct {
	BucketStart  string  `json:"bucket_start"`
	ForecastLow  int64   `json:"forecast_low"`
	ForecastMid  int64   `json:"forecast_mid"`
	ForecastHigh int64   `json:"forecast_high"`
	Confidence   float64 `json:"confidence"`
}

type SendVolumeForecastResult struct {
	Status      string                  `json:"status"`
	WorkspaceID string                  `json:"workspace_id"`
	Rows        []SendVolumeForecastRow `json:"rows"`
}

type AnomalyRow struct {
	AnomalyID   string  `json:"anomaly_id"`
	AnomalyType string  `json:"anomaly_type"`
	Severity    string  `json:"severity"`
	Metric      string  `json:"metric"`
	Observed    float64 `json:"observed"`
	Expected    float64 `json:"expected"`
	Deviation   float64 `json:"deviation"`
	DetectedAt  string  `json:"detected_at"`
	WindowStart string  `json:"window_start"`
	WindowEnd   string  `json:"window_end"`
}

type AnomalyWindowKey struct {
	WorkspaceID string
	AnomalyType string
	WindowStart time.Time
	WindowEnd   time.Time
}

func (k AnomalyWindowKey) String() string {
	return fmt.Sprintf("%s|%s|%s|%s", k.WorkspaceID, k.AnomalyType, k.WindowStart.Format(time.RFC3339), k.WindowEnd.Format(time.RFC3339))
}

type AnomaliesResult struct {
	Status      string       `json:"status"`
	WorkspaceID string       `json:"workspace_id"`
	Anomalies   []AnomalyRow `json:"anomalies"`
}

func ComputeRate(numerator, denominator int64) float64 {
	if denominator == 0 {
		return 0
	}
	return float64(numerator) / float64(denominator) * 100
}

type ForensicQueryFilter struct {
	WorkspaceID string
	From        time.Time
	To          time.Time
	EventType   string
	Provider    string
	Domain      string
	CampaignID  string
	MessageID   string
	Limit       int
	Cursor      string
}

func (f ForensicQueryFilter) Validate() error {
	if strings.TrimSpace(f.WorkspaceID) == "" {
		return fmt.Errorf("%w: workspace_id is required", ErrAnalyticsQueryInvalid)
	}
	if f.From.IsZero() && f.To.IsZero() && f.MessageID == "" {
		return fmt.Errorf("%w: search must be bounded by time range or message_id", ErrAnalyticsQueryInvalid)
	}
	if err := ValidateTimeRange(f.From, f.To); err != nil {
		return err
	}
	if f.EventType != "" && !KnownEventTypes[f.EventType] {
		return fmt.Errorf("%w: unsupported event_type %q", ErrAnalyticsQueryInvalid, f.EventType)
	}
	if f.Limit <= 0 || f.Limit > 100 {
		return fmt.Errorf("%w: limit must be between 1 and 100", ErrAnalyticsQueryInvalid)
	}
	return nil
}

type ForensicEventRow struct {
	SourceEventID     string `json:"source_event_id"`
	SourceEventType   string `json:"source_event_type"`
	WorkspaceID       string `json:"workspace_id"`
	CampaignID        string `json:"campaign_id,omitempty"`
	MessageID         string `json:"message_id,omitempty"`
	Provider          string `json:"provider,omitempty"`
	ProviderMessageID string `json:"provider_message_id,omitempty"`
	ProviderEventID   string `json:"provider_event_id,omitempty"`
	EventType         string `json:"event_type"`
	RecipientDomain   string `json:"recipient_domain,omitempty"`
	OccurredAt        string `json:"occurred_at"`
	ReceivedAt        string `json:"received_at"`
}

type ForensicEventsResult struct {
	Status      string             `json:"status"`
	WorkspaceID string             `json:"workspace_id"`
	Events      []ForensicEventRow `json:"events"`
	NextCursor  string             `json:"next_cursor,omitempty"`
	TotalCount  int64              `json:"total_count,omitempty"`
}

type MessageTimelineRow struct {
	SourceEventID     string `json:"source_event_id"`
	SourceEventType   string `json:"source_event_type"`
	EventType         string `json:"event_type"`
	Provider          string `json:"provider,omitempty"`
	ProviderMessageID string `json:"provider_message_id,omitempty"`
	ProviderEventID   string `json:"provider_event_id,omitempty"`
	CampaignID        string `json:"campaign_id,omitempty"`
	RecipientDomain   string `json:"recipient_domain,omitempty"`
	OccurredAt        string `json:"occurred_at"`
	ReceivedAt        string `json:"received_at"`
}

type MessageTimelineResult struct {
	Status      string               `json:"status"`
	WorkspaceID string               `json:"workspace_id"`
	MessageID   string               `json:"message_id"`
	Events      []MessageTimelineRow `json:"events"`
}

type ProviderEventTrace struct {
	Status            string `json:"status"`
	ProviderEventID   string `json:"provider_event_id"`
	WorkspaceID       string `json:"workspace_id,omitempty"`
	CampaignID        string `json:"campaign_id,omitempty"`
	MessageID         string `json:"message_id,omitempty"`
	Provider          string `json:"provider,omitempty"`
	ProviderMessageID string `json:"provider_message_id,omitempty"`
}

type CampaignIncidentTimelineRow struct {
	SourceEventID   string `json:"source_event_id"`
	SourceEventType string `json:"source_event_type"`
	EventType       string `json:"event_type"`
	MessageID       string `json:"message_id,omitempty"`
	Provider        string `json:"provider,omitempty"`
	RecipientDomain string `json:"recipient_domain,omitempty"`
	OccurredAt      string `json:"occurred_at"`
}

type CampaignIncidentTimelineResult struct {
	Status      string                        `json:"status"`
	WorkspaceID string                        `json:"workspace_id"`
	CampaignID  string                        `json:"campaign_id"`
	Events      []CampaignIncidentTimelineRow `json:"events"`
}

type OperationsQueryFilter struct {
	WorkspaceID   string
	From          time.Time
	To            time.Time
	Source        string
	Status        string
	OperationType string
	Limit         int
	Cursor        string
}

func (f OperationsQueryFilter) Validate() error {
	if strings.TrimSpace(f.WorkspaceID) == "" {
		return fmt.Errorf("%w: workspace_id is required", ErrAnalyticsQueryInvalid)
	}
	if err := ValidateTimeRange(f.From, f.To); err != nil {
		return err
	}
	if f.Limit <= 0 || f.Limit > 100 {
		return fmt.Errorf("%w: limit must be between 1 and 100", ErrAnalyticsQueryInvalid)
	}
	return nil
}

type OutboxLagRow struct {
	Source      string  `json:"source"`
	EventType   string  `json:"event_type"`
	LagSeconds  float64 `json:"lag_seconds"`
	Count       int64   `json:"count"`
	BucketStart string  `json:"bucket_start"`
}

type OutboxLagResult struct {
	Status      string         `json:"status"`
	WorkspaceID string         `json:"workspace_id"`
	Rows        []OutboxLagRow `json:"rows"`
}

type ConsumerFailureRow struct {
	Source      string `json:"source"`
	Consumer    string `json:"consumer"`
	ErrorType   string `json:"error_type"`
	Count       int64  `json:"count"`
	BucketStart string `json:"bucket_start"`
}

type ConsumerFailureResult struct {
	Status      string               `json:"status"`
	WorkspaceID string               `json:"workspace_id"`
	Rows        []ConsumerFailureRow `json:"rows"`
}

type DLQRow struct {
	Source      string `json:"source"`
	EventType   string `json:"event_type"`
	Count       int64  `json:"count"`
	BucketStart string `json:"bucket_start"`
}

type DLQResult struct {
	Status      string   `json:"status"`
	WorkspaceID string   `json:"workspace_id"`
	Rows        []DLQRow `json:"rows"`
}

type WebhookDeliveryTimeSeriesBucket struct {
	BucketStart string `json:"bucket_start"`
	Status      string `json:"status"`
	Count       int64  `json:"count"`
}

type WebhookDeliveryTimeSeriesResult struct {
	Status      string                            `json:"status"`
	WorkspaceID string                            `json:"workspace_id"`
	Buckets     []WebhookDeliveryTimeSeriesBucket `json:"buckets"`
}

type WebhookReliabilityRow struct {
	Source      string  `json:"source"`
	Target      string  `json:"target,omitempty"`
	TotalCount  int64   `json:"total_count"`
	Succeeded   int64   `json:"succeeded"`
	Failed      int64   `json:"failed"`
	Retried     int64   `json:"retried"`
	SuccessRate float64 `json:"success_rate"`
}

type WebhookReliabilityResult struct {
	Status      string                  `json:"status"`
	WorkspaceID string                  `json:"workspace_id"`
	Rows        []WebhookReliabilityRow `json:"rows"`
}
