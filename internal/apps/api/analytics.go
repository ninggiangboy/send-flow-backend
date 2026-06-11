package api

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	analyticsapp "github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/app"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/auth"
)

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
