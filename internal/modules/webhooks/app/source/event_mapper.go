package source

import (
	"encoding/json"
	"errors"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/webhooks/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/events"
	platformerrors "github.com/ninggiangboy/send-flow/backend/internal/platform/retryable"
)

var ErrUnsupportedEventType = errors.New("unsupported event type")
var ErrMalformedPayload = errors.New("malformed event payload")

type MappedSourceEvent struct {
	EventID     string
	EventType   string
	WorkspaceID string
	OccurredAt  string
	Payload     json.RawMessage
}

func MapEnvelopeToSourceEvent(envelope events.Envelope) (*MappedSourceEvent, error) {
	if !domain.AllowedSubscriptionEvents[envelope.EventType] {
		return nil, ErrUnsupportedEventType
	}
	if envelope.WorkspaceID == "" {
		return nil, &platformerrors.NonRetryableError{Err: ErrMalformedPayload}
	}
	return &MappedSourceEvent{
		EventID:     envelope.EventID,
		EventType:   envelope.EventType,
		WorkspaceID: envelope.WorkspaceID,
		OccurredAt:  envelope.OccurredAt.UTC().Format("2006-01-02T15:04:05Z"),
		Payload:     envelope.Payload,
	}, nil
}
