package kafka

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/ninggiangboy/send-flow/backend/internal/platform/events"
)

var ErrDisabled = errors.New("kafka is disabled")

type Message struct {
	Topic   string
	Key     []byte
	Value   []byte
	Headers map[string]string
}

type Producer interface {
	Produce(context.Context, Message) error
	Close() error
}

type Consumer interface {
	Consume(context.Context, Handler) error
	Close() error
}

type Handler func(context.Context, Message) error

func Brokers(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	brokers := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			brokers = append(brokers, part)
		}
	}
	return brokers
}

func NewMessageFromEnvelope(env events.Envelope) (Message, error) {
	value, err := events.Marshal(env)
	if err != nil {
		return Message{}, err
	}
	msg := Message{
		Topic: events.TopicFromEventType(env.EventType),
		Key:   []byte(env.AggregateID),
		Value: value,
		Headers: map[string]string{
			"event_id":        env.EventID,
			"event_type":      env.EventType,
			"aggregate_type":  env.AggregateType,
			"aggregate_id":    env.AggregateID,
			"event_version":   strconv.Itoa(env.EventVersion),
			"content_type":    "application/json",
			"content_schema":  env.EventType,
			"platform_source": "send-flow",
		},
	}
	if env.WorkspaceID != "" {
		msg.Headers["workspace_id"] = env.WorkspaceID
	}
	return msg, nil
}

type DisabledProducer struct{}

func (DisabledProducer) Produce(context.Context, Message) error {
	return ErrDisabled
}

func (DisabledProducer) Close() error {
	return nil
}
