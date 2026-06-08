package domain

import (
	"encoding/json"
	"time"
)

const (
	EventTypeAccepted        = "accepted"
	EventTypeDelivered       = "delivered"
	EventTypeDelayed         = "delayed"
	EventTypeBounced         = "bounced"
	EventTypeComplained      = "complained"
	EventTypeOpened          = "opened"
	EventTypeClicked         = "clicked"
	EventTypeUnsubscribed    = "unsubscribed"
	EventTypeRejected        = "rejected"
	EventTypeRenderingFailed = "rendering_failed"
)

const (
	ProviderFake = "fake"
	ProviderSES  = "ses"
)

func ValidEventType(s string) bool {
	switch s {
	case EventTypeAccepted, EventTypeDelivered, EventTypeDelayed,
		EventTypeBounced, EventTypeComplained, EventTypeOpened,
		EventTypeClicked, EventTypeUnsubscribed, EventTypeRejected,
		EventTypeRenderingFailed:
		return true
	default:
		return false
	}
}

type ProviderWebhookEvent struct {
	ID                string
	Provider          string
	ProviderEventID   string
	ProviderMessageID string
	WorkspaceID       string
	MessageID         string
	EventType         string
	PayloadJSON       json.RawMessage
	HeadersJSON       json.RawMessage
	SignatureValid    bool
	ReceivedAt        time.Time
	CreatedAt         time.Time
}

type NormalizedProviderEvent struct {
	ID                string
	RawEventID        string
	WorkspaceID       string
	MessageID         string
	Provider          string
	ProviderEventID   string
	ProviderMessageID string
	EventType         string
	OccurredAt        time.Time
	ReceivedAt        time.Time
	PayloadJSON       json.RawMessage
	CreatedAt         time.Time
}

func ValidateRawEvent(e ProviderWebhookEvent) error {
	if e.Provider == "" {
		return ErrPayloadInvalid
	}
	if len(e.PayloadJSON) == 0 {
		return ErrPayloadInvalid
	}
	if e.ReceivedAt.IsZero() {
		return ErrPayloadInvalid
	}
	return nil
}

func ValidateNormalizedEvent(e NormalizedProviderEvent) error {
	if e.Provider == "" {
		return ErrPayloadInvalid
	}
	if e.RawEventID == "" {
		return ErrPayloadInvalid
	}
	if !ValidEventType(e.EventType) {
		return ErrPayloadInvalid
	}
	if e.OccurredAt.IsZero() {
		return ErrPayloadInvalid
	}
	if e.ReceivedAt.IsZero() {
		return ErrPayloadInvalid
	}
	return nil
}
