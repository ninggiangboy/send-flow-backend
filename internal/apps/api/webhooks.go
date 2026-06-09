package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	webhooksapp "github.com/ninggiangboy/send-flow/backend/internal/modules/webhooks/app"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/webhooks/domain"
)

type webhookConfigHTTP struct {
	svc *webhooksapp.Service
}

func newWebhookHTTP(svc *webhooksapp.Service) *webhookConfigHTTP {
	return &webhookConfigHTTP{svc: svc}
}

type createWebhookConfigRequest struct {
	Name          string   `json:"name"`
	TargetURL     string   `json:"target_url"`
	Subscriptions []string `json:"subscribed_events"`
}

type webhookConfigResponse struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	TargetURL        string    `json:"target_url"`
	Status           string    `json:"status"`
	SubscribedEvents []string  `json:"subscribed_events"`
	Secret           string    `json:"secret,omitempty"`
	Version          int64     `json:"version"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type webhookConfigListResponse struct {
	Data []webhookConfigListItem `json:"data"`
	Meta requestMetaDoc          `json:"meta"`
}

type webhookConfigListItem struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	TargetURL        string    `json:"target_url"`
	Status           string    `json:"status"`
	SubscribedEvents []string  `json:"subscribed_events"`
	SecretHint       string    `json:"secret_hint"`
	Version          int64     `json:"version"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type updateWebhookConfigRequest struct {
	Name          *string  `json:"name,omitempty"`
	TargetURL     *string  `json:"target_url,omitempty"`
	Subscriptions []string `json:"subscribed_events,omitempty"`
	Status        *string  `json:"status,omitempty"`
}

func (h *webhookConfigHTTP) listWebhookConfigs(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	userID := r.Context().Value(ctxUserID).(string)

	configs, err := h.svc.ListWebhookConfigs(r.Context(), webhooksapp.ListConfigsInput{
		WorkspaceID: workspaceID,
		UserID:      userID,
	})
	if err != nil {
		mapWebhookConfigErr(w, r, err)
		return
	}

	items := make([]webhookConfigListItem, 0, len(configs))
	for _, c := range configs {
		items = append(items, webhookConfigListItem{
			ID:               c.ID,
			Name:             c.Name,
			TargetURL:        c.TargetURL,
			Status:           string(c.Status),
			SubscribedEvents: c.Subscriptions,
			SecretHint:       c.SecretHint,
			Version:          c.Version,
			CreatedAt:        c.CreatedAt,
			UpdatedAt:        c.UpdatedAt,
		})
	}

	writeEnvelope(w, r, http.StatusOK, webhookConfigListResponse{
		Data: items,
		Meta: requestMetaDoc{RequestID: requestID(r)},
	})
}

func (h *webhookConfigHTTP) createWebhookConfig(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	userID := r.Context().Value(ctxUserID).(string)

	var req createWebhookConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "auth.invalid_request_body", "invalid request body", nil)
		return
	}

	result, err := h.svc.CreateWebhookConfig(r.Context(), webhooksapp.CreateConfigInput{
		WorkspaceID:   workspaceID,
		UserID:        userID,
		Name:          req.Name,
		TargetURL:     req.TargetURL,
		Subscriptions: req.Subscriptions,
		Now:           time.Now().UTC(),
	})
	if err != nil {
		mapWebhookConfigErr(w, r, err)
		return
	}

	writeEnvelope(w, r, http.StatusCreated, webhookConfigResponse{
		ID:               result.Config.ID,
		Name:             result.Config.Name,
		TargetURL:        result.Config.TargetURL,
		Status:           string(result.Config.Status),
		SubscribedEvents: result.Config.Subscriptions,
		Secret:           result.RawSecret,
		Version:          result.Config.Version,
		CreatedAt:        result.Config.CreatedAt,
		UpdatedAt:        result.Config.UpdatedAt,
	})
}

func (h *webhookConfigHTTP) updateWebhookConfig(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	webhookID := chi.URLParam(r, "webhook_id")
	userID := r.Context().Value(ctxUserID).(string)

	var req updateWebhookConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, "auth.invalid_request_body", "invalid request body", nil)
		return
	}

	result, err := h.svc.UpdateWebhookConfig(r.Context(), webhooksapp.UpdateConfigInput{
		WorkspaceID:   workspaceID,
		UserID:        userID,
		WebhookID:     webhookID,
		Name:          req.Name,
		TargetURL:     req.TargetURL,
		Subscriptions: req.Subscriptions,
		Status:        req.Status,
		Now:           time.Now().UTC(),
	})
	if err != nil {
		mapWebhookConfigErr(w, r, err)
		return
	}

	writeEnvelope(w, r, http.StatusOK, webhookConfigResponse{
		ID:               result.Config.ID,
		Name:             result.Config.Name,
		TargetURL:        result.Config.TargetURL,
		Status:           string(result.Config.Status),
		SubscribedEvents: result.Config.Subscriptions,
		Version:          result.Config.Version,
		CreatedAt:        result.Config.CreatedAt,
		UpdatedAt:        result.Config.UpdatedAt,
	})
}

func (h *webhookConfigHTTP) disableWebhookConfig(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	webhookID := chi.URLParam(r, "webhook_id")
	userID := r.Context().Value(ctxUserID).(string)

	if err := h.svc.DisableWebhookConfig(r.Context(), webhooksapp.DisableConfigInput{
		WorkspaceID: workspaceID,
		UserID:      userID,
		WebhookID:   webhookID,
	}); err != nil {
		mapWebhookConfigErr(w, r, err)
		return
	}

	writeEnvelope(w, r, http.StatusOK, map[string]bool{"disabled": true})
}

func (h *webhookConfigHTTP) rotateWebhookSecret(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	webhookID := chi.URLParam(r, "webhook_id")
	userID := r.Context().Value(ctxUserID).(string)

	result, err := h.svc.RotateWebhookSecret(r.Context(), webhooksapp.RotateSecretInput{
		WorkspaceID: workspaceID,
		UserID:      userID,
		WebhookID:   webhookID,
	})
	if err != nil {
		mapWebhookConfigErr(w, r, err)
		return
	}

	writeEnvelope(w, r, http.StatusOK, map[string]any{
		"secret":  result.RawSecret,
		"hint":    result.Hint,
		"version": result.Version,
	})
}

type webhookDeliveryHTTP struct {
	svc *webhooksapp.Service
}

func newWebhookDeliveryHTTP(svc *webhooksapp.Service) *webhookDeliveryHTTP {
	return &webhookDeliveryHTTP{svc: svc}
}

type deliveryListItem struct {
	ID              string     `json:"id"`
	WebhookID       string     `json:"webhook_id"`
	SourceEventID   string     `json:"source_event_id"`
	SourceEventType string     `json:"source_event_type"`
	Status          string     `json:"status"`
	TargetURL       string     `json:"target_url"`
	AttemptCount    int64      `json:"attempt_count"`
	LastAttemptAt   *time.Time `json:"last_attempt_at,omitempty"`
	LastStatusCode  *int       `json:"last_status_code,omitempty"`
	LastError       string     `json:"last_error,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

type deliveryListResponse struct {
	Data []deliveryListItem `json:"data"`
	Meta requestMetaDoc     `json:"meta"`
	Next string             `json:"next,omitempty"`
}

type deliveryDetailResponse struct {
	Data deliveryDetailDoc `json:"data"`
	Meta requestMetaDoc    `json:"meta"`
}

type deliveryDetailDoc struct {
	ID              string               `json:"id"`
	WebhookID       string               `json:"webhook_id"`
	SourceEventID   string               `json:"source_event_id"`
	SourceEventType string               `json:"source_event_type"`
	Status          string               `json:"status"`
	TargetURL       string               `json:"target_url"`
	AttemptCount    int64                `json:"attempt_count"`
	LastAttemptAt   *time.Time           `json:"last_attempt_at,omitempty"`
	LastStatusCode  *int                 `json:"last_status_code,omitempty"`
	LastError       string               `json:"last_error,omitempty"`
	CreatedAt       time.Time            `json:"created_at"`
	UpdatedAt       time.Time            `json:"updated_at"`
	Attempts        []deliveryAttemptDoc `json:"attempts,omitempty"`
}

type deliveryAttemptDoc struct {
	ID            string    `json:"id"`
	AttemptNumber int64     `json:"attempt_number"`
	Status        string    `json:"status"`
	StatusCode    *int      `json:"status_code,omitempty"`
	Error         string    `json:"error,omitempty"`
	DurationMs    int64     `json:"duration_ms"`
	AttemptedAt   time.Time `json:"attempted_at"`
}

func (h *webhookDeliveryHTTP) listWebhookDeliveries(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	userID := r.Context().Value(ctxUserID).(string)

	q := r.URL.Query()
	var from, to *time.Time
	if v := q.Get("from"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err == nil {
			from = &t
		}
	}
	if v := q.Get("to"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err == nil {
			to = &t
		}
	}

	var limit int
	parseLimit(q.Get("limit"), &limit)

	result, err := h.svc.ListWebhookDeliveries(r.Context(), webhooksapp.ListDeliveriesInput{
		WorkspaceID: workspaceID,
		UserID:      userID,
		WebhookID:   q.Get("webhook_id"),
		Status:      q.Get("status"),
		EventType:   q.Get("event_type"),
		From:        from,
		To:          to,
		Limit:       limit,
		Cursor:      q.Get("cursor"),
	})
	if err != nil {
		mapWebhookDeliveryErr(w, r, err)
		return
	}

	items := make([]deliveryListItem, 0, len(result.Deliveries))
	for _, d := range result.Deliveries {
		items = append(items, deliveryListItem{
			ID:              d.ID,
			WebhookID:       d.WebhookID,
			SourceEventID:   d.SourceEventID,
			SourceEventType: d.SourceEventType,
			Status:          string(d.Status),
			TargetURL:       d.TargetURL,
			AttemptCount:    d.AttemptCount,
			LastAttemptAt:   d.LastAttemptAt,
			LastStatusCode:  d.LastStatusCode,
			LastError:       d.LastError,
			CreatedAt:       d.CreatedAt,
		})
	}

	writeEnvelope(w, r, http.StatusOK, deliveryListResponse{
		Data: items,
		Meta: requestMetaDoc{RequestID: requestID(r)},
		Next: result.NextCursor,
	})
}

func (h *webhookDeliveryHTTP) getWebhookDelivery(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	deliveryID := chi.URLParam(r, "delivery_id")
	userID := r.Context().Value(ctxUserID).(string)

	delivery, err := h.svc.GetWebhookDelivery(r.Context(), webhooksapp.GetDeliveryInput{
		WorkspaceID: workspaceID,
		UserID:      userID,
		DeliveryID:  deliveryID,
	})
	if err != nil {
		mapWebhookDeliveryErr(w, r, err)
		return
	}

	attempts := make([]deliveryAttemptDoc, 0, len(delivery.Attempts))
	for _, a := range delivery.Attempts {
		attempts = append(attempts, deliveryAttemptDoc{
			ID:            a.ID,
			AttemptNumber: a.AttemptNumber,
			Status:        a.Status,
			StatusCode:    a.StatusCode,
			Error:         a.Error,
			DurationMs:    a.DurationMs,
			AttemptedAt:   a.AttemptedAt,
		})
	}

	writeEnvelope(w, r, http.StatusOK, deliveryDetailResponse{
		Data: deliveryDetailDoc{
			ID:              delivery.ID,
			WebhookID:       delivery.WebhookID,
			SourceEventID:   delivery.SourceEventID,
			SourceEventType: delivery.SourceEventType,
			Status:          string(delivery.Status),
			TargetURL:       delivery.TargetURL,
			AttemptCount:    delivery.AttemptCount,
			LastAttemptAt:   delivery.LastAttemptAt,
			LastStatusCode:  delivery.LastStatusCode,
			LastError:       delivery.LastError,
			CreatedAt:       delivery.CreatedAt,
			UpdatedAt:       delivery.UpdatedAt,
			Attempts:        attempts,
		},
		Meta: requestMetaDoc{RequestID: requestID(r)},
	})
}

func (h *webhookDeliveryHTTP) retryWebhookDelivery(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	deliveryID := chi.URLParam(r, "delivery_id")
	userID := r.Context().Value(ctxUserID).(string)

	if err := h.svc.RetryWebhookDelivery(r.Context(), webhooksapp.RetryDeliveryInput{
		WorkspaceID: workspaceID,
		UserID:      userID,
		DeliveryID:  deliveryID,
	}); err != nil {
		mapWebhookDeliveryErr(w, r, err)
		return
	}

	writeEnvelope(w, r, http.StatusOK, map[string]string{"status": "retrying"})
}

func mapWebhookConfigErr(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrManageDenied):
		writeError(w, r, http.StatusForbidden, "webhook.manage_denied", err.Error(), nil)
	case errors.Is(err, domain.ErrConfigNotFound):
		writeError(w, r, http.StatusNotFound, "webhook.config_not_found", err.Error(), nil)
	case errors.Is(err, domain.ErrTargetURLInvalid):
		writeError(w, r, http.StatusUnprocessableEntity, "webhook.target_url_invalid", err.Error(), nil)
	case errors.Is(err, domain.ErrSubscriptionInvalid):
		writeError(w, r, http.StatusUnprocessableEntity, "webhook.subscription_invalid", err.Error(), nil)
	case errors.Is(err, domain.ErrConfigInvalid):
		writeError(w, r, http.StatusUnprocessableEntity, "webhook.config_invalid", err.Error(), nil)
	case errors.Is(err, domain.ErrConfigNameConflict):
		writeError(w, r, http.StatusConflict, "webhook.config_name_conflict", err.Error(), nil)
	case errors.Is(err, domain.ErrRotateConflict):
		writeError(w, r, http.StatusConflict, "webhook.rotate_conflict", err.Error(), nil)
	default:
		writeError(w, r, http.StatusInternalServerError, "health.runtime_not_ready", err.Error(), nil)
	}
}

func mapWebhookDeliveryErr(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrDeliveryReadDenied):
		writeError(w, r, http.StatusForbidden, "webhook.delivery_read_denied", err.Error(), nil)
	case errors.Is(err, domain.ErrDeliveryRetryDenied):
		writeError(w, r, http.StatusForbidden, "webhook.delivery_retry_denied", err.Error(), nil)
	case errors.Is(err, domain.ErrDeliveryNotFound):
		writeError(w, r, http.StatusNotFound, "webhook.delivery_not_found", err.Error(), nil)
	case errors.Is(err, domain.ErrRetryConflict):
		writeError(w, r, http.StatusConflict, "webhook.delivery_retry_conflict", err.Error(), nil)
	case errors.Is(err, domain.ErrConfigNotFound):
		writeError(w, r, http.StatusNotFound, "webhook.config_not_found", err.Error(), nil)
	case errors.Is(err, domain.ErrConfigDisabled):
		writeError(w, r, http.StatusConflict, "webhook.config_disabled", err.Error(), nil)
	default:
		writeError(w, r, http.StatusInternalServerError, "health.runtime_not_ready", err.Error(), nil)
	}
}
