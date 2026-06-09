package app

import (
	"encoding/json"
	"errors"

	"github.com/ninggiangboy/send-flow/backend/internal/platform/events"
)

var ErrUnsupportedEventType = errors.New("unsupported event type")
var ErrMalformedPayload = errors.New("malformed event payload")

var supportedSourceEventTypes = map[string]bool{
	"delivery.message.queued.v1":          true,
	"delivery.message.accepted.v1":        true,
	"delivery.message.delivered.v1":       true,
	"delivery.message.bounced.v1":         true,
	"delivery.message.complained.v1":      true,
	"delivery.message.retry_scheduled.v1": true,
	"tracking.email_opened.v1":            true,
	"tracking.link_clicked.v1":            true,
	"tracking.recipient_unsubscribed.v1":  true,
	"suppression.recipient_suppressed.v1": true,
}

type MappedSourceEvent struct {
	EventID     string
	EventType   string
	WorkspaceID string
	OccurredAt  string
	Payload     json.RawMessage
}

func MapEnvelopeToSourceEvent(envelope events.Envelope) (*MappedSourceEvent, error) {
	if !supportedSourceEventTypes[envelope.EventType] {
		return nil, ErrUnsupportedEventType
	}
	if envelope.WorkspaceID == "" {
		return nil, &NonRetryableError{Err: ErrMalformedPayload}
	}
	return &MappedSourceEvent{
		EventID:     envelope.EventID,
		EventType:   envelope.EventType,
		WorkspaceID: envelope.WorkspaceID,
		OccurredAt:  envelope.OccurredAt.UTC().Format("2006-01-02T15:04:05Z"),
		Payload:     envelope.Payload,
	}, nil
}
