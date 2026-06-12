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
	Status      string                     `json:"status"`
	WorkspaceID string                     `json:"workspace_id"`
	CampaignID  string                     `json:"campaign_id"`
	GroupBy     string                     `json:"group_by"`
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
