package shared

import (
	"errors"

	"github.com/ninggiangboy/send-flow/backend/internal/platform/events"
)

var ErrUnsupportedEventType = errors.New("unsupported event type")

type EventMapper func(envelope events.Envelope) (*MappedEvent, error)

type MapperRegistry struct {
	mappers map[string]EventMapper
}

func NewMapperRegistry() *MapperRegistry {
	return &MapperRegistry{mappers: make(map[string]EventMapper)}
}

func (r *MapperRegistry) Register(eventType string, mapper EventMapper) {
	r.mappers[eventType] = mapper
}

func (r *MapperRegistry) MapEvent(envelope events.Envelope) (*MappedEvent, error) {
	mapper, ok := r.mappers[envelope.EventType]
	if !ok {
		return nil, ErrUnsupportedEventType
	}
	return mapper(envelope)
}

func (r *MapperRegistry) EventTypes() []string {
	types := make([]string, 0, len(r.mappers))
	for t := range r.mappers {
		types = append(types, t)
	}
	return types
}
