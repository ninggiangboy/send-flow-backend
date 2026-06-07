package ports

import (
	"context"
	"time"
)

type OutboxEvent struct {
	ID            string
	AggregateType string
	AggregateID   string
	EventType     string
	Payload       []byte
	Headers       map[string]string
	WorkspaceID   string
	OccurredAt    time.Time
}

type OutboxWriter interface {
	Save(ctx context.Context, event OutboxEvent) error
}
