package outbox

import (
	"context"
	"time"
)

type Event struct {
	ID            string
	AggregateType string
	AggregateID   string
	EventType     string
	Payload       []byte
	Headers       map[string]string
	WorkspaceID   string
	OccurredAt    time.Time
}

type Writer interface {
	Save(ctx context.Context, event Event) error
}
