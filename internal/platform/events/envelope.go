package events

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrMissingEventID   = errors.New("event id is required")
	ErrMissingEventType = errors.New("event type is required")
	ErrMissingAggregate = errors.New("aggregate id is required")
)

type Envelope struct {
	EventID       string          `json:"event_id"`
	EventType     string          `json:"event_type"`
	EventVersion  int             `json:"event_version"`
	AggregateType string          `json:"aggregate_type"`
	AggregateID   string          `json:"aggregate_id"`
	WorkspaceID   string          `json:"workspace_id,omitempty"`
	OccurredAt    time.Time       `json:"occurred_at"`
	Payload       json.RawMessage `json:"payload"`
	Metadata      Metadata        `json:"metadata,omitempty"`
}

type Metadata map[string]string

type NewEnvelopeOptions struct {
	EventID       string
	EventType     string
	EventVersion  int
	AggregateType string
	AggregateID   string
	WorkspaceID   string
	OccurredAt    time.Time
	Metadata      Metadata
}

func NewEnvelope(opts NewEnvelopeOptions, payload any) (Envelope, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return Envelope{}, fmt.Errorf("marshal event payload: %w", err)
	}

	env := Envelope{
		EventID:       opts.EventID,
		EventType:     opts.EventType,
		EventVersion:  opts.EventVersion,
		AggregateType: opts.AggregateType,
		AggregateID:   opts.AggregateID,
		WorkspaceID:   opts.WorkspaceID,
		OccurredAt:    opts.OccurredAt.UTC(),
		Payload:       raw,
		Metadata:      opts.Metadata,
	}
	if env.EventVersion == 0 {
		env.EventVersion = 1
	}
	if env.OccurredAt.IsZero() {
		env.OccurredAt = time.Now().UTC()
	}
	if env.Metadata == nil {
		env.Metadata = Metadata{}
	}
	if err := env.Validate(); err != nil {
		return Envelope{}, err
	}
	return env, nil
}

func (e Envelope) Validate() error {
	if strings.TrimSpace(e.EventID) == "" {
		return ErrMissingEventID
	}
	if strings.TrimSpace(e.EventType) == "" {
		return ErrMissingEventType
	}
	if strings.TrimSpace(e.AggregateID) == "" {
		return ErrMissingAggregate
	}
	if e.EventVersion < 1 {
		return errors.New("event version must be positive")
	}
	if len(e.Payload) == 0 {
		return errors.New("event payload is required")
	}
	return nil
}

func (e Envelope) DecodePayload(dst any) error {
	if len(e.Payload) == 0 {
		return errors.New("event payload is empty")
	}
	return json.Unmarshal(e.Payload, dst)
}

func Marshal(e Envelope) ([]byte, error) {
	if err := e.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(e)
}

func Unmarshal(data []byte) (Envelope, error) {
	var e Envelope
	if err := json.Unmarshal(data, &e); err != nil {
		return Envelope{}, err
	}
	if err := e.Validate(); err != nil {
		return Envelope{}, err
	}
	return e, nil
}

func TopicFromEventType(eventType string) string {
	if idx := strings.LastIndex(eventType, ".v"); idx > 0 {
		suffix := eventType[idx+2:]
		for _, r := range suffix {
			if r < '0' || r > '9' {
				return eventType
			}
		}
		return eventType[:idx]
	}
	return eventType
}
