package api

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	analyticsapp "github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/app"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/auth"
)

func parseTimePtr(s string) *time.Time {
	if s == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil
	}
	return &t
}

type analyticsHTTP struct {
	svc *analyticsapp.Service
}

func newAnalyticsHTTP(svc *analyticsapp.Service) *analyticsHTTP {
	return &analyticsHTTP{svc: svc}
}

type overviewResponseDoc struct {
	Status              string  `json:"status"`
	WorkspaceID         string  `json:"workspace_id"`
	QueuedCount         int64   `json:"queued_count"`
	AcceptedCount       int64   `json:"accepted_count"`
	DeliveredCount      int64   `json:"delivered_count"`
	BouncedCount        int64   `json:"bounced_count"`
	ComplainedCount     int64   `json:"complained_count"`
	OpenedCount         int64   `json:"opened_count"`
	ClickedCount        int64   `json:"clicked_count"`
	UnsubscribedCount   int64   `json:"unsubscribed_count"`
	RetryScheduledCount int64   `json:"retry_scheduled_count"`
	LastEventAt         *string `json:"last_event_at,omitempty"`
	LastUpdatedAt       *string `json:"last_updated_at,omitempty"`
}

type campaignAnalyticsResponseDoc struct {
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
	LastEventAt         *string `json:"last_event_at,omitempty"`
	LastUpdatedAt       *string `json:"last_updated_at,omitempty"`
}

type deliverabilityRowDoc struct {
	Provider        string  `json:"provider"`
	RecipientDomain string  `json:"recipient_domain"`
	DeliveredCount  int64   `json:"delivered_count"`
	BouncedCount    int64   `json:"bounced_count"`
	ComplainedCount int64   `json:"complained_count"`
	OpenedCount     int64   `json:"opened_count"`
	ClickedCount    int64   `json:"clicked_count"`
	BounceRate      float64 `json:"bounce_rate"`
	ComplaintRate   float64 `json:"complaint_rate"`
	LastEventAt     *string `json:"last_event_at,omitempty"`
	LastUpdatedAt   *string `json:"last_updated_at,omitempty"`
}

type deliverabilityResponseDoc struct {
	Status string                 `json:"status"`
	Items  []deliverabilityRowDoc `json:"items,omitempty"`
}

func timePtrToStringPtr(t *string) *string {
	if t == nil || *t == "" {
		return nil
	}
	return t
}

func (h *analyticsHTTP) getDashboardOverview(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	userID, _ := r.Context().Value(ctxUserID).(string)

	result, err := h.svc.GetDashboardOverview(r.Context(), analyticsapp.GetDashboardOverviewInput{
		WorkspaceID: workspaceID,
		UserID:      userID,
	})
	if err != nil {
		writeAnalyticsErr(w, r, err)
		return
	}

	resp := overviewResponseDoc{
		Status:              result.Status,
		WorkspaceID:         result.WorkspaceID,
		QueuedCount:         result.QueuedCount,
		AcceptedCount:       result.AcceptedCount,
		DeliveredCount:      result.DeliveredCount,
		BouncedCount:        result.BouncedCount,
		ComplainedCount:     result.ComplainedCount,
		OpenedCount:         result.OpenedCount,
		ClickedCount:        result.ClickedCount,
		UnsubscribedCount:   result.UnsubscribedCount,
		RetryScheduledCount: result.RetryScheduledCount,
	}
	if result.LastEventAt != nil {
		t := result.LastEventAt.Format("2006-01-02T15:04:05Z07:00")
		resp.LastEventAt = &t
	}
	t := result.LastUpdatedAt.Format("2006-01-02T15:04:05Z07:00")
	resp.LastUpdatedAt = &t

	writeEnvelope(w, r, http.StatusOK, resp)
}

func (h *analyticsHTTP) getCampaignAnalytics(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	campaignID := chi.URLParam(r, "campaign_id")
	userID, _ := r.Context().Value(ctxUserID).(string)

	result, err := h.svc.GetCampaignAnalytics(r.Context(), analyticsapp.GetCampaignAnalyticsInput{
		WorkspaceID: workspaceID,
		CampaignID:  campaignID,
		UserID:      userID,
	})
	if err != nil {
		writeAnalyticsErr(w, r, err)
		return
	}

	resp := campaignAnalyticsResponseDoc{
		Status:              result.Status,
		WorkspaceID:         result.WorkspaceID,
		CampaignID:          result.CampaignID,
		QueuedCount:         result.QueuedCount,
		AcceptedCount:       result.AcceptedCount,
		DeliveredCount:      result.DeliveredCount,
		BouncedCount:        result.BouncedCount,
		ComplainedCount:     result.ComplainedCount,
		OpenedCount:         result.OpenedCount,
		ClickedCount:        result.ClickedCount,
		UnsubscribedCount:   result.UnsubscribedCount,
		RetryScheduledCount: result.RetryScheduledCount,
		DeliveryRate:        result.DeliveryRate,
		BounceRate:          result.BounceRate,
		ComplaintRate:       result.ComplaintRate,
		OpenRate:            result.OpenRate,
		ClickRate:           result.ClickRate,
		UnsubscribeRate:     result.UnsubscribeRate,
	}
	if result.LastEventAt != nil {
		t := result.LastEventAt.Format("2006-01-02T15:04:05Z07:00")
		resp.LastEventAt = &t
	}
	t := result.LastUpdatedAt.Format("2006-01-02T15:04:05Z07:00")
	resp.LastUpdatedAt = &t

	writeEnvelope(w, r, http.StatusOK, resp)
}

func (h *analyticsHTTP) getDeliverability(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	userID, _ := r.Context().Value(ctxUserID).(string)
	q := r.URL.Query()

	result, err := h.svc.GetDeliverability(r.Context(), analyticsapp.GetDeliverabilityInput{
		WorkspaceID:     workspaceID,
		Provider:        q.Get("provider"),
		RecipientDomain: q.Get("recipient_domain"),
		UserID:          userID,
	})
	if err != nil {
		writeAnalyticsErr(w, r, err)
		return
	}

	resp := deliverabilityResponseDoc{
		Status: result.Status,
		Items:  make([]deliverabilityRowDoc, 0, len(result.Items)),
	}
	for _, item := range result.Items {
		row := deliverabilityRowDoc{
			Provider:        item.Provider,
			RecipientDomain: item.RecipientDomain,
			DeliveredCount:  item.DeliveredCount,
			BouncedCount:    item.BouncedCount,
			ComplainedCount: item.ComplainedCount,
			OpenedCount:     item.OpenedCount,
			ClickedCount:    item.ClickedCount,
			BounceRate:      item.BounceRate,
			ComplaintRate:   item.ComplaintRate,
		}
		if item.LastEventAt != nil {
			t := item.LastEventAt.Format("2006-01-02T15:04:05Z07:00")
			row.LastEventAt = &t
		}
		t := item.LastUpdatedAt.Format("2006-01-02T15:04:05Z07:00")
		row.LastUpdatedAt = &t
		resp.Items = append(resp.Items, row)
	}

	writeEnvelope(w, r, http.StatusOK, resp)
}

type deliverabilityTimeSeriesBucketDoc struct {
	BucketStart     string `json:"bucket_start"`
	Provider        string `json:"provider,omitempty"`
	RecipientDomain string `json:"recipient_domain,omitempty"`
	EventType       string `json:"event_type"`
	Count           int64  `json:"count"`
}

type deliverabilityTimeSeriesResponseDoc struct {
	Status      string                              `json:"status"`
	WorkspaceID string                              `json:"workspace_id"`
	Buckets     []deliverabilityTimeSeriesBucketDoc `json:"buckets"`
}

type deliverabilityBreakdownRowDoc struct {
	Provider        string  `json:"provider,omitempty"`
	RecipientDomain string  `json:"recipient_domain,omitempty"`
	EventType       string  `json:"event_type"`
	Count           int64   `json:"count"`
	Rate            float64 `json:"rate"`
	LastEventAt     string  `json:"last_event_at,omitempty"`
}

type deliverabilityBreakdownResponseDoc struct {
	Status      string                          `json:"status"`
	WorkspaceID string                          `json:"workspace_id"`
	GroupBy     string                          `json:"group_by"`
	Rows        []deliverabilityBreakdownRowDoc `json:"rows"`
}

type deliverabilityLatencyRowDoc struct {
	Provider        string  `json:"provider"`
	RecipientDomain string  `json:"recipient_domain,omitempty"`
	EventType       string  `json:"event_type"`
	Count           int64   `json:"count"`
	P50LatencyMs    float64 `json:"p50_latency_ms"`
	P95LatencyMs    float64 `json:"p95_latency_ms"`
	P99LatencyMs    float64 `json:"p99_latency_ms"`
	AvgLatencyMs    float64 `json:"avg_latency_ms"`
}

type deliverabilityLatencyResponseDoc struct {
	Status      string                        `json:"status"`
	WorkspaceID string                        `json:"workspace_id"`
	Rows        []deliverabilityLatencyRowDoc `json:"rows"`
}

type deliverabilityIncidentRowDoc struct {
	Provider        string  `json:"provider"`
	RecipientDomain string  `json:"recipient_domain,omitempty"`
	EventType       string  `json:"event_type"`
	IncidentStart   string  `json:"incident_start"`
	IncidentEnd     string  `json:"incident_end,omitempty"`
	EventCount      int64   `json:"event_count"`
	Rate            float64 `json:"rate"`
}

type deliverabilityIncidentResponseDoc struct {
	Status      string                         `json:"status"`
	WorkspaceID string                         `json:"workspace_id"`
	Provider    string                         `json:"provider,omitempty"`
	Domain      string                         `json:"domain,omitempty"`
	Rows        []deliverabilityIncidentRowDoc `json:"rows"`
}

type campaignFunnelResponseDoc struct {
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

type campaignTimeSeriesBucketDoc struct {
	BucketStart string `json:"bucket_start"`
	EventType   string `json:"event_type"`
	Count       int64  `json:"count"`
}

type campaignTimeSeriesResponseDoc struct {
	Status      string                        `json:"status"`
	WorkspaceID string                        `json:"workspace_id"`
	CampaignID  string                        `json:"campaign_id"`
	Buckets     []campaignTimeSeriesBucketDoc `json:"buckets"`
}

type campaignBreakdownRowDoc struct {
	GroupKey    string  `json:"group_key"`
	EventType   string  `json:"event_type"`
	Count       int64   `json:"count"`
	Rate        float64 `json:"rate"`
	LastEventAt string  `json:"last_event_at,omitempty"`
}

type campaignBreakdownResponseDoc struct {
	Status      string                    `json:"status"`
	WorkspaceID string                    `json:"workspace_id"`
	CampaignID  string                    `json:"campaign_id"`
	GroupBy     string                    `json:"group_by"`
	Rows        []campaignBreakdownRowDoc `json:"rows"`
}

type campaignEventRowDoc struct {
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

type campaignEventsResponseDoc struct {
	Status      string                `json:"status"`
	WorkspaceID string                `json:"workspace_id"`
	CampaignID  string                `json:"campaign_id"`
	Events      []campaignEventRowDoc `json:"events"`
	NextCursor  string                `json:"next_cursor,omitempty"`
}

type forensicEventRowDoc struct {
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

type forensicEventsResponseDoc struct {
	Status      string                `json:"status"`
	WorkspaceID string                `json:"workspace_id"`
	Events      []forensicEventRowDoc `json:"events"`
	NextCursor  string                `json:"next_cursor,omitempty"`
}

type messageTimelineRowDoc struct {
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

type messageTimelineResponseDoc struct {
	Status      string                  `json:"status"`
	WorkspaceID string                  `json:"workspace_id"`
	MessageID   string                  `json:"message_id"`
	Events      []messageTimelineRowDoc `json:"events"`
}

type providerEventTraceResponseDoc struct {
	Status            string `json:"status"`
	ProviderEventID   string `json:"provider_event_id"`
	WorkspaceID       string `json:"workspace_id,omitempty"`
	CampaignID        string `json:"campaign_id,omitempty"`
	MessageID         string `json:"message_id,omitempty"`
	Provider          string `json:"provider,omitempty"`
	ProviderMessageID string `json:"provider_message_id,omitempty"`
}

type campaignIncidentTimelineRowDoc struct {
	SourceEventID   string `json:"source_event_id"`
	SourceEventType string `json:"source_event_type"`
	EventType       string `json:"event_type"`
	MessageID       string `json:"message_id,omitempty"`
	Provider        string `json:"provider,omitempty"`
	RecipientDomain string `json:"recipient_domain,omitempty"`
	OccurredAt      string `json:"occurred_at"`
}

type campaignIncidentTimelineResponseDoc struct {
	Status      string                           `json:"status"`
	WorkspaceID string                           `json:"workspace_id"`
	CampaignID  string                           `json:"campaign_id"`
	Events      []campaignIncidentTimelineRowDoc `json:"events"`
}

func (h *analyticsHTTP) getCampaignFunnel(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	campaignID := chi.URLParam(r, "campaign_id")
	userID, _ := r.Context().Value(ctxUserID).(string)
	q := r.URL.Query()

	var from, to time.Time
	if v := parseTimePtr(q.Get("from")); v != nil {
		from = *v
	}
	if v := parseTimePtr(q.Get("to")); v != nil {
		to = *v
	}

	result, err := h.svc.GetCampaignFunnel(r.Context(), analyticsapp.GetCampaignFunnelInput{
		WorkspaceID: workspaceID,
		CampaignID:  campaignID,
		UserID:      userID,
		From:        from,
		To:          to,
	})
	if err != nil {
		writeAnalyticsErr(w, r, err)
		return
	}

	resp := campaignFunnelResponseDoc{
		Status:              result.Status,
		WorkspaceID:         result.WorkspaceID,
		CampaignID:          result.CampaignID,
		QueuedCount:         result.QueuedCount,
		AcceptedCount:       result.AcceptedCount,
		DeliveredCount:      result.DeliveredCount,
		BouncedCount:        result.BouncedCount,
		ComplainedCount:     result.ComplainedCount,
		OpenedCount:         result.OpenedCount,
		ClickedCount:        result.ClickedCount,
		UnsubscribedCount:   result.UnsubscribedCount,
		RetryScheduledCount: result.RetryScheduledCount,
		DeliveryRate:        result.DeliveryRate,
		BounceRate:          result.BounceRate,
		ComplaintRate:       result.ComplaintRate,
		OpenRate:            result.OpenRate,
		ClickRate:           result.ClickRate,
		UnsubscribeRate:     result.UnsubscribeRate,
		LastEventAt:         result.LastEventAt,
	}

	writeEnvelope(w, r, http.StatusOK, resp)
}

func (h *analyticsHTTP) getCampaignTimeSeries(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	campaignID := chi.URLParam(r, "campaign_id")
	userID, _ := r.Context().Value(ctxUserID).(string)
	q := r.URL.Query()

	var from, to time.Time
	if v := parseTimePtr(q.Get("from")); v != nil {
		from = *v
	}
	if v := parseTimePtr(q.Get("to")); v != nil {
		to = *v
	}
	interval := q.Get("interval")
	if interval == "" {
		interval = "day"
	}
	eventType := q.Get("event_type")

	result, err := h.svc.GetCampaignTimeSeries(r.Context(), analyticsapp.GetCampaignTimeSeriesInput{
		WorkspaceID: workspaceID,
		CampaignID:  campaignID,
		UserID:      userID,
		From:        from,
		To:          to,
		Interval:    interval,
		EventType:   eventType,
	})
	if err != nil {
		writeAnalyticsErr(w, r, err)
		return
	}

	buckets := make([]campaignTimeSeriesBucketDoc, 0, len(result.Buckets))
	for _, b := range result.Buckets {
		buckets = append(buckets, campaignTimeSeriesBucketDoc{
			BucketStart: b.BucketStart.Format(time.RFC3339),
			EventType:   b.EventType,
			Count:       b.Count,
		})
	}

	resp := campaignTimeSeriesResponseDoc{
		Status:      result.Status,
		WorkspaceID: result.WorkspaceID,
		CampaignID:  result.CampaignID,
		Buckets:     buckets,
	}

	writeEnvelope(w, r, http.StatusOK, resp)
}

func (h *analyticsHTTP) getCampaignBreakdown(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	campaignID := chi.URLParam(r, "campaign_id")
	userID, _ := r.Context().Value(ctxUserID).(string)
	q := r.URL.Query()

	var from, to time.Time
	if v := parseTimePtr(q.Get("from")); v != nil {
		from = *v
	}
	if v := parseTimePtr(q.Get("to")); v != nil {
		to = *v
	}
	groupBy := q.Get("group_by")
	if groupBy == "" {
		groupBy = "event_type"
	}

	result, err := h.svc.GetCampaignBreakdown(r.Context(), analyticsapp.GetCampaignBreakdownInput{
		WorkspaceID: workspaceID,
		CampaignID:  campaignID,
		UserID:      userID,
		From:        from,
		To:          to,
		GroupBy:     groupBy,
	})
	if err != nil {
		writeAnalyticsErr(w, r, err)
		return
	}

	deliveredByGroup := make(map[string]int64)
	totalDelivered := int64(0)
	for _, row := range result.Rows {
		if row.EventType == "delivered" {
			deliveredByGroup[row.GroupKey] = row.Count
			totalDelivered += row.Count
		}
	}

	rows := make([]campaignBreakdownRowDoc, 0, len(result.Rows))
	for _, row := range result.Rows {
		denominator := deliveredByGroup[row.GroupKey]
		if result.GroupBy == "event_type" {
			denominator = totalDelivered
		}
		rows = append(rows, campaignBreakdownRowDoc{
			GroupKey:    row.GroupKey,
			EventType:   row.EventType,
			Count:       row.Count,
			Rate:        domain.ComputeRate(row.Count, denominator),
			LastEventAt: row.LastEventAt,
		})
	}

	resp := campaignBreakdownResponseDoc{
		Status:      result.Status,
		WorkspaceID: result.WorkspaceID,
		CampaignID:  result.CampaignID,
		GroupBy:     result.GroupBy,
		Rows:        rows,
	}

	writeEnvelope(w, r, http.StatusOK, resp)
}

func (h *analyticsHTTP) getCampaignEvents(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	campaignID := chi.URLParam(r, "campaign_id")
	userID, _ := r.Context().Value(ctxUserID).(string)
	q := r.URL.Query()

	var from, to time.Time
	if v := parseTimePtr(q.Get("from")); v != nil {
		from = *v
	}
	if v := parseTimePtr(q.Get("to")); v != nil {
		to = *v
	}
	eventType := q.Get("event_type")
	provider := q.Get("provider")
	domain := q.Get("recipient_domain")
	cursor := q.Get("cursor")

	limit := 50
	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}

	result, err := h.svc.GetCampaignEvents(r.Context(), analyticsapp.GetCampaignEventsInput{
		WorkspaceID: workspaceID,
		CampaignID:  campaignID,
		UserID:      userID,
		From:        from,
		To:          to,
		EventType:   eventType,
		Provider:    provider,
		Domain:      domain,
		Limit:       limit,
		Cursor:      cursor,
	})
	if err != nil {
		writeAnalyticsErr(w, r, err)
		return
	}

	events := make([]campaignEventRowDoc, 0, len(result.Events))
	for _, e := range result.Events {
		events = append(events, campaignEventRowDoc{
			SourceEventID:     e.SourceEventID,
			SourceEventType:   e.SourceEventType,
			MessageID:         e.MessageID,
			Provider:          e.Provider,
			ProviderMessageID: e.ProviderMessageID,
			EventType:         e.EventType,
			RecipientDomain:   e.RecipientDomain,
			OccurredAt:        e.OccurredAt,
			ReceivedAt:        e.ReceivedAt,
		})
	}

	resp := campaignEventsResponseDoc{
		Status:      result.Status,
		WorkspaceID: result.WorkspaceID,
		CampaignID:  result.CampaignID,
		Events:      events,
		NextCursor:  result.NextCursor,
	}

	writeEnvelope(w, r, http.StatusOK, resp)
}

func (h *analyticsHTTP) searchEvents(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	userID, _ := r.Context().Value(ctxUserID).(string)
	q := r.URL.Query()

	var from, to time.Time
	if v := parseTimePtr(q.Get("from")); v != nil {
		from = *v
	}
	if v := parseTimePtr(q.Get("to")); v != nil {
		to = *v
	}
	eventType := q.Get("event_type")
	provider := q.Get("provider")
	domain := q.Get("recipient_domain")
	campaignID := q.Get("campaign_id")
	messageID := q.Get("message_id")
	cursor := q.Get("cursor")

	limit := 50
	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}

	result, err := h.svc.SearchEvents(r.Context(), analyticsapp.SearchEventsInput{
		WorkspaceID: workspaceID,
		UserID:      userID,
		From:        from,
		To:          to,
		EventType:   eventType,
		Provider:    provider,
		Domain:      domain,
		CampaignID:  campaignID,
		MessageID:   messageID,
		Limit:       limit,
		Cursor:      cursor,
	})
	if err != nil {
		writeAnalyticsErr(w, r, err)
		return
	}

	events := make([]forensicEventRowDoc, 0, len(result.Events))
	for _, e := range result.Events {
		events = append(events, forensicEventRowDoc{
			SourceEventID:     e.SourceEventID,
			SourceEventType:   e.SourceEventType,
			WorkspaceID:       e.WorkspaceID,
			CampaignID:        e.CampaignID,
			MessageID:         e.MessageID,
			Provider:          e.Provider,
			ProviderMessageID: e.ProviderMessageID,
			ProviderEventID:   e.ProviderEventID,
			EventType:         e.EventType,
			RecipientDomain:   e.RecipientDomain,
			OccurredAt:        e.OccurredAt,
			ReceivedAt:        e.ReceivedAt,
		})
	}

	resp := forensicEventsResponseDoc{
		Status:      result.Status,
		WorkspaceID: result.WorkspaceID,
		Events:      events,
		NextCursor:  result.NextCursor,
	}

	writeEnvelope(w, r, http.StatusOK, resp)
}

func (h *analyticsHTTP) getMessageTimeline(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	messageID := chi.URLParam(r, "message_id")
	userID, _ := r.Context().Value(ctxUserID).(string)

	result, err := h.svc.GetMessageTimeline(r.Context(), analyticsapp.GetMessageTimelineInput{
		WorkspaceID: workspaceID,
		UserID:      userID,
		MessageID:   messageID,
	})
	if err != nil {
		writeAnalyticsErr(w, r, err)
		return
	}

	events := make([]messageTimelineRowDoc, 0, len(result.Events))
	for _, e := range result.Events {
		events = append(events, messageTimelineRowDoc{
			SourceEventID:     e.SourceEventID,
			SourceEventType:   e.SourceEventType,
			EventType:         e.EventType,
			Provider:          e.Provider,
			ProviderMessageID: e.ProviderMessageID,
			ProviderEventID:   e.ProviderEventID,
			CampaignID:        e.CampaignID,
			RecipientDomain:   e.RecipientDomain,
			OccurredAt:        e.OccurredAt,
			ReceivedAt:        e.ReceivedAt,
		})
	}

	resp := messageTimelineResponseDoc{
		Status:      result.Status,
		WorkspaceID: result.WorkspaceID,
		MessageID:   result.MessageID,
		Events:      events,
	}

	writeEnvelope(w, r, http.StatusOK, resp)
}

func (h *analyticsHTTP) getProviderEventTrace(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	providerEventID := chi.URLParam(r, "provider_event_id")
	userID, _ := r.Context().Value(ctxUserID).(string)

	result, err := h.svc.GetProviderEventTrace(r.Context(), analyticsapp.GetProviderEventTraceInput{
		WorkspaceID:     workspaceID,
		UserID:          userID,
		ProviderEventID: providerEventID,
	})
	if err != nil {
		writeAnalyticsErr(w, r, err)
		return
	}

	if result == nil {
		writeError(w, r, http.StatusNotFound, "analytics.projection_not_found", "provider event not found", nil)
		return
	}

	resp := providerEventTraceResponseDoc{
		Status:            result.Status,
		ProviderEventID:   result.ProviderEventID,
		WorkspaceID:       result.WorkspaceID,
		CampaignID:        result.CampaignID,
		MessageID:         result.MessageID,
		Provider:          result.Provider,
		ProviderMessageID: result.ProviderMessageID,
	}

	writeEnvelope(w, r, http.StatusOK, resp)
}

func (h *analyticsHTTP) getCampaignIncidentTimeline(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	campaignID := chi.URLParam(r, "campaign_id")
	userID, _ := r.Context().Value(ctxUserID).(string)
	q := r.URL.Query()

	var from, to time.Time
	if v := parseTimePtr(q.Get("from")); v != nil {
		from = *v
	}
	if v := parseTimePtr(q.Get("to")); v != nil {
		to = *v
	}

	result, err := h.svc.GetCampaignIncidentTimeline(r.Context(), analyticsapp.GetCampaignIncidentTimelineInput{
		WorkspaceID: workspaceID,
		CampaignID:  campaignID,
		UserID:      userID,
		From:        from,
		To:          to,
	})
	if err != nil {
		writeAnalyticsErr(w, r, err)
		return
	}

	events := make([]campaignIncidentTimelineRowDoc, 0, len(result.Events))
	for _, e := range result.Events {
		events = append(events, campaignIncidentTimelineRowDoc{
			SourceEventID:   e.SourceEventID,
			SourceEventType: e.SourceEventType,
			EventType:       e.EventType,
			MessageID:       e.MessageID,
			Provider:        e.Provider,
			RecipientDomain: e.RecipientDomain,
			OccurredAt:      e.OccurredAt,
		})
	}

	resp := campaignIncidentTimelineResponseDoc{
		Status:      result.Status,
		WorkspaceID: result.WorkspaceID,
		CampaignID:  result.CampaignID,
		Events:      events,
	}

	writeEnvelope(w, r, http.StatusOK, resp)
}

func (h *analyticsHTTP) getDeliverabilityTimeSeries(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	userID, _ := r.Context().Value(ctxUserID).(string)
	q := r.URL.Query()

	var from, to time.Time
	if v := parseTimePtr(q.Get("from")); v != nil {
		from = *v
	}
	if v := parseTimePtr(q.Get("to")); v != nil {
		to = *v
	}
	interval := q.Get("interval")
	if interval == "" {
		interval = "day"
	}
	provider := q.Get("provider")
	domain := q.Get("recipient_domain")

	result, err := h.svc.GetDeliverabilityTimeSeries(r.Context(), analyticsapp.GetDeliverabilityTimeSeriesInput{
		WorkspaceID: workspaceID,
		UserID:      userID,
		From:        from,
		To:          to,
		Interval:    interval,
		Provider:    provider,
		Domain:      domain,
	})
	if err != nil {
		writeAnalyticsErr(w, r, err)
		return
	}

	buckets := make([]deliverabilityTimeSeriesBucketDoc, 0, len(result.Buckets))
	for _, b := range result.Buckets {
		buckets = append(buckets, deliverabilityTimeSeriesBucketDoc{
			BucketStart:     b.BucketStart.Format(time.RFC3339),
			Provider:        b.Provider,
			RecipientDomain: b.RecipientDomain,
			EventType:       b.EventType,
			Count:           b.Count,
		})
	}

	resp := deliverabilityTimeSeriesResponseDoc{
		Status:      result.Status,
		WorkspaceID: result.WorkspaceID,
		Buckets:     buckets,
	}

	writeEnvelope(w, r, http.StatusOK, resp)
}

func (h *analyticsHTTP) getDeliverabilityBreakdown(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	userID, _ := r.Context().Value(ctxUserID).(string)
	q := r.URL.Query()

	var from, to time.Time
	if v := parseTimePtr(q.Get("from")); v != nil {
		from = *v
	}
	if v := parseTimePtr(q.Get("to")); v != nil {
		to = *v
	}
	groupBy := q.Get("group_by")
	if groupBy == "" {
		groupBy = "provider"
	}
	provider := q.Get("provider")
	domain := q.Get("recipient_domain")

	result, err := h.svc.GetDeliverabilityBreakdown(r.Context(), analyticsapp.GetDeliverabilityBreakdownInput{
		WorkspaceID: workspaceID,
		UserID:      userID,
		From:        from,
		To:          to,
		GroupBy:     groupBy,
		Provider:    provider,
		Domain:      domain,
	})
	if err != nil {
		writeAnalyticsErr(w, r, err)
		return
	}

	rows := make([]deliverabilityBreakdownRowDoc, 0, len(result.Rows))
	for _, row := range result.Rows {
		rows = append(rows, deliverabilityBreakdownRowDoc{
			Provider:        row.Provider,
			RecipientDomain: row.RecipientDomain,
			EventType:       row.EventType,
			Count:           row.Count,
			Rate:            row.Rate,
			LastEventAt:     row.LastEventAt,
		})
	}

	resp := deliverabilityBreakdownResponseDoc{
		Status:      result.Status,
		WorkspaceID: result.WorkspaceID,
		GroupBy:     result.GroupBy,
		Rows:        rows,
	}

	writeEnvelope(w, r, http.StatusOK, resp)
}

func (h *analyticsHTTP) getDeliverabilityLatency(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	userID, _ := r.Context().Value(ctxUserID).(string)
	q := r.URL.Query()

	var from, to time.Time
	if v := parseTimePtr(q.Get("from")); v != nil {
		from = *v
	}
	if v := parseTimePtr(q.Get("to")); v != nil {
		to = *v
	}
	provider := q.Get("provider")
	domain := q.Get("recipient_domain")

	result, err := h.svc.GetDeliverabilityLatency(r.Context(), analyticsapp.GetDeliverabilityLatencyInput{
		WorkspaceID: workspaceID,
		UserID:      userID,
		From:        from,
		To:          to,
		Provider:    provider,
		Domain:      domain,
	})
	if err != nil {
		writeAnalyticsErr(w, r, err)
		return
	}

	rows := make([]deliverabilityLatencyRowDoc, 0, len(result.Rows))
	for _, row := range result.Rows {
		rows = append(rows, deliverabilityLatencyRowDoc{
			Provider:        row.Provider,
			RecipientDomain: row.RecipientDomain,
			EventType:       row.EventType,
			Count:           row.Count,
			P50LatencyMs:    row.P50LatencyMs,
			P95LatencyMs:    row.P95LatencyMs,
			P99LatencyMs:    row.P99LatencyMs,
			AvgLatencyMs:    row.AvgLatencyMs,
		})
	}

	resp := deliverabilityLatencyResponseDoc{
		Status:      result.Status,
		WorkspaceID: result.WorkspaceID,
		Rows:        rows,
	}

	writeEnvelope(w, r, http.StatusOK, resp)
}

func (h *analyticsHTTP) getDeliverabilityIncidents(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	userID, _ := r.Context().Value(ctxUserID).(string)
	q := r.URL.Query()

	var from, to time.Time
	if v := parseTimePtr(q.Get("from")); v != nil {
		from = *v
	}
	if v := parseTimePtr(q.Get("to")); v != nil {
		to = *v
	}
	provider := q.Get("provider")
	domain := q.Get("recipient_domain")

	result, err := h.svc.GetDeliverabilityIncidents(r.Context(), analyticsapp.GetDeliverabilityIncidentsInput{
		WorkspaceID: workspaceID,
		UserID:      userID,
		From:        from,
		To:          to,
		Provider:    provider,
		Domain:      domain,
	})
	if err != nil {
		writeAnalyticsErr(w, r, err)
		return
	}

	rows := make([]deliverabilityIncidentRowDoc, 0, len(result.Rows))
	for _, row := range result.Rows {
		rows = append(rows, deliverabilityIncidentRowDoc{
			Provider:        row.Provider,
			RecipientDomain: row.RecipientDomain,
			EventType:       row.EventType,
			IncidentStart:   row.IncidentStart,
			IncidentEnd:     row.IncidentEnd,
			EventCount:      row.EventCount,
			Rate:            row.Rate,
		})
	}

	resp := deliverabilityIncidentResponseDoc{
		Status:      result.Status,
		WorkspaceID: result.WorkspaceID,
		Provider:    result.Provider,
		Domain:      result.Domain,
		Rows:        rows,
	}

	writeEnvelope(w, r, http.StatusOK, resp)
}

type outboxLagRowDoc struct {
	Source      string  `json:"source"`
	EventType   string  `json:"event_type"`
	LagSeconds  float64 `json:"lag_seconds"`
	Count       int64   `json:"count"`
	BucketStart string  `json:"bucket_start"`
}

type outboxLagResponseDoc struct {
	Status      string            `json:"status"`
	WorkspaceID string            `json:"workspace_id"`
	Rows        []outboxLagRowDoc `json:"rows"`
}

type consumerFailureRowDoc struct {
	Source      string `json:"source"`
	Consumer    string `json:"consumer"`
	ErrorType   string `json:"error_type"`
	Count       int64  `json:"count"`
	BucketStart string `json:"bucket_start"`
}

type consumerFailureResponseDoc struct {
	Status      string                  `json:"status"`
	WorkspaceID string                  `json:"workspace_id"`
	Rows        []consumerFailureRowDoc `json:"rows"`
}

type dlqRowDoc struct {
	Source      string `json:"source"`
	EventType   string `json:"event_type"`
	Count       int64  `json:"count"`
	BucketStart string `json:"bucket_start"`
}

type dlqResponseDoc struct {
	Status      string      `json:"status"`
	WorkspaceID string      `json:"workspace_id"`
	Rows        []dlqRowDoc `json:"rows"`
}

type webhookDeliveryTimeSeriesBucketDoc struct {
	BucketStart string `json:"bucket_start"`
	Status      string `json:"status"`
	Count       int64  `json:"count"`
}

type webhookDeliveryTimeSeriesResponseDoc struct {
	Status      string                               `json:"status"`
	WorkspaceID string                               `json:"workspace_id"`
	Buckets     []webhookDeliveryTimeSeriesBucketDoc `json:"buckets"`
}

type webhookReliabilityRowDoc struct {
	Source      string  `json:"source"`
	Target      string  `json:"target,omitempty"`
	TotalCount  int64   `json:"total_count"`
	Succeeded   int64   `json:"succeeded"`
	Failed      int64   `json:"failed"`
	Retried     int64   `json:"retried"`
	SuccessRate float64 `json:"success_rate"`
}

type webhookReliabilityResponseDoc struct {
	Status      string                     `json:"status"`
	WorkspaceID string                     `json:"workspace_id"`
	Rows        []webhookReliabilityRowDoc `json:"rows"`
}

func (h *analyticsHTTP) getOutboxLag(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	userID, _ := r.Context().Value(ctxUserID).(string)
	q := r.URL.Query()

	var from, to time.Time
	if v := parseTimePtr(q.Get("from")); v != nil {
		from = *v
	}
	if v := parseTimePtr(q.Get("to")); v != nil {
		to = *v
	}
	source := q.Get("source")

	result, err := h.svc.GetOutboxLag(r.Context(), analyticsapp.GetOutboxLagInput{
		WorkspaceID: workspaceID,
		UserID:      userID,
		From:        from,
		To:          to,
		Source:      source,
	})
	if err != nil {
		writeAnalyticsErr(w, r, err)
		return
	}

	rows := make([]outboxLagRowDoc, 0, len(result.Rows))
	for _, row := range result.Rows {
		rows = append(rows, outboxLagRowDoc{
			Source:      row.Source,
			EventType:   row.EventType,
			LagSeconds:  row.LagSeconds,
			Count:       row.Count,
			BucketStart: row.BucketStart,
		})
	}

	writeEnvelope(w, r, http.StatusOK, outboxLagResponseDoc{
		Status:      result.Status,
		WorkspaceID: result.WorkspaceID,
		Rows:        rows,
	})
}

func (h *analyticsHTTP) getConsumerFailures(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	userID, _ := r.Context().Value(ctxUserID).(string)
	q := r.URL.Query()

	var from, to time.Time
	if v := parseTimePtr(q.Get("from")); v != nil {
		from = *v
	}
	if v := parseTimePtr(q.Get("to")); v != nil {
		to = *v
	}
	source := q.Get("source")

	result, err := h.svc.GetConsumerFailures(r.Context(), analyticsapp.GetConsumerFailuresInput{
		WorkspaceID: workspaceID,
		UserID:      userID,
		From:        from,
		To:          to,
		Source:      source,
	})
	if err != nil {
		writeAnalyticsErr(w, r, err)
		return
	}

	rows := make([]consumerFailureRowDoc, 0, len(result.Rows))
	for _, row := range result.Rows {
		rows = append(rows, consumerFailureRowDoc{
			Source:      row.Source,
			Consumer:    row.Consumer,
			ErrorType:   row.ErrorType,
			Count:       row.Count,
			BucketStart: row.BucketStart,
		})
	}

	writeEnvelope(w, r, http.StatusOK, consumerFailureResponseDoc{
		Status:      result.Status,
		WorkspaceID: result.WorkspaceID,
		Rows:        rows,
	})
}

func (h *analyticsHTTP) getDLQVolume(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	userID, _ := r.Context().Value(ctxUserID).(string)
	q := r.URL.Query()

	var from, to time.Time
	if v := parseTimePtr(q.Get("from")); v != nil {
		from = *v
	}
	if v := parseTimePtr(q.Get("to")); v != nil {
		to = *v
	}
	source := q.Get("source")

	result, err := h.svc.GetDLQVolume(r.Context(), analyticsapp.GetDLQVolumeInput{
		WorkspaceID: workspaceID,
		UserID:      userID,
		From:        from,
		To:          to,
		Source:      source,
	})
	if err != nil {
		writeAnalyticsErr(w, r, err)
		return
	}

	rows := make([]dlqRowDoc, 0, len(result.Rows))
	for _, row := range result.Rows {
		rows = append(rows, dlqRowDoc{
			Source:      row.Source,
			EventType:   row.EventType,
			Count:       row.Count,
			BucketStart: row.BucketStart,
		})
	}

	writeEnvelope(w, r, http.StatusOK, dlqResponseDoc{
		Status:      result.Status,
		WorkspaceID: result.WorkspaceID,
		Rows:        rows,
	})
}

func (h *analyticsHTTP) getWebhookDeliveryTimeSeries(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	userID, _ := r.Context().Value(ctxUserID).(string)
	q := r.URL.Query()

	var from, to time.Time
	if v := parseTimePtr(q.Get("from")); v != nil {
		from = *v
	}
	if v := parseTimePtr(q.Get("to")); v != nil {
		to = *v
	}
	status := q.Get("status")
	interval := q.Get("interval")
	if interval == "" {
		interval = "day"
	}

	result, err := h.svc.GetWebhookDeliveryTimeSeries(r.Context(), analyticsapp.GetWebhookDeliveryTimeSeriesInput{
		WorkspaceID: workspaceID,
		UserID:      userID,
		From:        from,
		To:          to,
		Status:      status,
		Interval:    interval,
	})
	if err != nil {
		writeAnalyticsErr(w, r, err)
		return
	}

	buckets := make([]webhookDeliveryTimeSeriesBucketDoc, 0, len(result.Buckets))
	for _, b := range result.Buckets {
		buckets = append(buckets, webhookDeliveryTimeSeriesBucketDoc{
			BucketStart: b.BucketStart,
			Status:      b.Status,
			Count:       b.Count,
		})
	}

	writeEnvelope(w, r, http.StatusOK, webhookDeliveryTimeSeriesResponseDoc{
		Status:      result.Status,
		WorkspaceID: result.WorkspaceID,
		Buckets:     buckets,
	})
}

func (h *analyticsHTTP) getWebhookReliability(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	userID, _ := r.Context().Value(ctxUserID).(string)
	q := r.URL.Query()

	var from, to time.Time
	if v := parseTimePtr(q.Get("from")); v != nil {
		from = *v
	}
	if v := parseTimePtr(q.Get("to")); v != nil {
		to = *v
	}
	target := q.Get("target")

	result, err := h.svc.GetWebhookReliability(r.Context(), analyticsapp.GetWebhookReliabilityInput{
		WorkspaceID: workspaceID,
		UserID:      userID,
		From:        from,
		To:          to,
		Target:      target,
	})
	if err != nil {
		writeAnalyticsErr(w, r, err)
		return
	}

	rows := make([]webhookReliabilityRowDoc, 0, len(result.Rows))
	for _, row := range result.Rows {
		rows = append(rows, webhookReliabilityRowDoc{
			Source:      row.Source,
			Target:      row.Target,
			TotalCount:  row.TotalCount,
			Succeeded:   row.Succeeded,
			Failed:      row.Failed,
			Retried:     row.Retried,
			SuccessRate: row.SuccessRate,
		})
	}

	writeEnvelope(w, r, http.StatusOK, webhookReliabilityResponseDoc{
		Status:      result.Status,
		WorkspaceID: result.WorkspaceID,
		Rows:        rows,
	})
}

func writeAnalyticsErr(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrAnalyticsQueryInvalid):
		writeError(w, r, http.StatusBadRequest, "analytics.query_invalid", err.Error(), nil)
	case errors.Is(err, domain.ErrAnalyticsProjectionNotFound):
		writeError(w, r, http.StatusNotFound, "analytics.projection_not_found", err.Error(), nil)
	case errors.Is(err, domain.ErrAnalyticsReadDenied):
		writeError(w, r, http.StatusForbidden, "analytics.read_denied", err.Error(), nil)
	case errors.Is(err, auth.ErrPermissionDenied):
		writeError(w, r, http.StatusForbidden, "auth.permission_denied", err.Error(), nil)
	default:
		writeError(w, r, http.StatusInternalServerError, "internal.error", "internal error", nil)
	}
}
