package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/notification/app"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/notification/domain"
)

type notificationHTTP struct {
	svc *app.Service
}

func newNotificationHTTP(svc *app.Service) *notificationHTTP {
	return &notificationHTTP{svc: svc}
}

type notificationListItem struct {
	ID              string  `json:"id"`
	WorkspaceID     *string `json:"workspace_id,omitempty"`
	Type            string  `json:"type"`
	Status          string  `json:"status"`
	RecipientEmail  string  `json:"recipient_email"`
	RecipientUserID *string `json:"recipient_user_id,omitempty"`
	Subject         string  `json:"subject"`
	MaxAttempts     int     `json:"max_attempts"`
	AttemptCount    int     `json:"attempt_count"`
	LastAttemptAt   *string `json:"last_attempt_at,omitempty"`
	CreatedAt       string  `json:"created_at"`
	UpdatedAt       string  `json:"updated_at"`
}

type notificationDetailItem struct {
	notificationListItem
	Attempts []attemptListItem `json:"attempts"`
}

type attemptListItem struct {
	ID                    string  `json:"id"`
	NotificationMessageID string  `json:"notification_message_id"`
	AttemptNumber         int     `json:"attempt_number"`
	Status                string  `json:"status"`
	Provider              string  `json:"provider"`
	ProviderMessageID     *string `json:"provider_message_id,omitempty"`
	ErrorMessage          *string `json:"error_message,omitempty"`
	AttemptedAt           string  `json:"attempted_at"`
}

type notificationListData struct {
	Notifications []notificationListItem `json:"notifications"`
	NextCursor    string                 `json:"next_cursor"`
}

type notificationDetailData struct {
	Notification notificationDetailItem `json:"notification"`
}

type systemAlertInput struct {
	RecipientEmail string `json:"recipient_email"`
	Subject        string `json:"subject"`
	Body           string `json:"body"`
}

func notificationToListItem(msg domain.NotificationMessage) notificationListItem {
	item := notificationListItem{
		ID:              msg.ID,
		WorkspaceID:     msg.WorkspaceID,
		Type:            string(msg.Type),
		Status:          string(msg.Status),
		RecipientEmail:  msg.RecipientEmail,
		RecipientUserID: msg.RecipientUserID,
		Subject:         msg.Subject,
		MaxAttempts:     msg.MaxAttempts,
		AttemptCount:    msg.AttemptCount,
		CreatedAt:       msg.CreatedAt.Format(time.RFC3339Nano),
		UpdatedAt:       msg.UpdatedAt.Format(time.RFC3339Nano),
	}
	if msg.LastAttemptAt != nil {
		s := msg.LastAttemptAt.Format(time.RFC3339Nano)
		item.LastAttemptAt = &s
	}
	return item
}

func attemptToListItem(a domain.NotificationAttempt) attemptListItem {
	item := attemptListItem{
		ID:                    a.ID,
		NotificationMessageID: a.NotificationMessageID,
		AttemptNumber:         a.AttemptNumber,
		Status:                string(a.Status),
		Provider:              a.Provider,
		ProviderMessageID:     a.ProviderMessageID,
		ErrorMessage:          a.ErrorMessage,
		AttemptedAt:           a.AttemptedAt.Format(time.RFC3339Nano),
	}
	return item
}

func (h *notificationHTTP) listNotifications(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	userID, _ := r.Context().Value(ctxUserID).(string)

	if workspaceID == "" {
		writeError(w, r, http.StatusBadRequest, "notification.filter_invalid", "workspace_id is required", nil)
		return
	}

	filter := domain.NotificationFilter{
		WorkspaceID: &workspaceID,
		Type:        r.URL.Query().Get("type"),
		Status:      r.URL.Query().Get("status"),
		Cursor:      r.URL.Query().Get("cursor"),
		Limit:       50,
	}

	if fromStr := r.URL.Query().Get("from"); fromStr != "" {
		from, err := time.Parse(time.RFC3339, fromStr)
		if err != nil {
			writeError(w, r, http.StatusBadRequest, "notification.filter_invalid", "invalid from timestamp", nil)
			return
		}
		filter.From = &from
	}
	if toStr := r.URL.Query().Get("to"); toStr != "" {
		to, err := time.Parse(time.RFC3339, toStr)
		if err != nil {
			writeError(w, r, http.StatusBadRequest, "notification.filter_invalid", "invalid to timestamp", nil)
			return
		}
		filter.To = &to
	}
	if filter.From != nil && filter.To != nil && filter.From.After(*filter.To) {
		writeError(w, r, http.StatusBadRequest, "notification.filter_invalid", "from must be before to", nil)
		return
	}

	result, err := h.svc.ListNotifications(r.Context(), filter, userID)
	if err != nil {
		writeNotificationErr(w, r, err)
		return
	}

	items := make([]notificationListItem, len(result.Messages))
	for i, msg := range result.Messages {
		items[i] = notificationToListItem(msg)
	}

	writeEnvelope(w, r, http.StatusOK, notificationListData{
		Notifications: items,
		NextCursor:    result.NextCursor,
	})
}

func (h *notificationHTTP) getNotificationStatus(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	notificationID := chi.URLParam(r, "notification_id")
	userID, _ := r.Context().Value(ctxUserID).(string)

	if workspaceID == "" || notificationID == "" {
		writeError(w, r, http.StatusBadRequest, "notification.filter_invalid", "workspace_id and notification_id are required", nil)
		return
	}

	result, err := h.svc.GetNotificationStatus(r.Context(), workspaceID, notificationID, userID)
	if err != nil {
		writeNotificationErr(w, r, err)
		return
	}

	item := notificationToListItem(result.Message)
	attempts := make([]attemptListItem, len(result.Attempts))
	for i, a := range result.Attempts {
		attempts[i] = attemptToListItem(a)
	}

	writeEnvelope(w, r, http.StatusOK, notificationDetailData{
		Notification: notificationDetailItem{
			notificationListItem: item,
			Attempts:             attempts,
		},
	})
}

func (h *notificationHTTP) sendSystemAlert(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")

	var input systemAlertInput
	if !decodeJSON(w, r, &input) {
		return
	}

	domainInput := domain.SendSystemAlertInput{
		RecipientEmail: input.RecipientEmail,
		Subject:        input.Subject,
		Body:           input.Body,
		WorkspaceID:    workspaceID,
	}

	msg, err := h.svc.SendSystemAlert(r.Context(), domainInput)
	if err != nil {
		writeNotificationErr(w, r, err)
		return
	}

	writeEnvelope(w, r, http.StatusCreated, notificationDetailData{
		Notification: notificationDetailItem{
			notificationListItem: notificationToListItem(*msg),
			Attempts:             nil,
		},
	})
}

func writeNotificationErr(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrNotificationReadDenied):
		writeError(w, r, http.StatusForbidden, "notification.read_denied", err.Error(), nil)
	case errors.Is(err, domain.ErrNotificationNotFound):
		writeError(w, r, http.StatusNotFound, "notification.not_found", err.Error(), nil)
	case errors.Is(err, domain.ErrRecipientEmailInvalid),
		errors.Is(err, domain.ErrSubjectInvalid),
		errors.Is(err, domain.ErrNotificationTypeInvalid),
		errors.Is(err, domain.ErrNotificationStatusInvalid):
		writeError(w, r, http.StatusBadRequest, "notification.filter_invalid", err.Error(), nil)
	default:
		writeError(w, r, http.StatusInternalServerError, "health.runtime_not_ready", "internal error", nil)
	}
}
