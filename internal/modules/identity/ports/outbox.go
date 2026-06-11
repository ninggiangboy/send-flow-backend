package ports

import (
	"context"

	"github.com/ninggiangboy/send-flow/backend/internal/platform/outbox"
)

type OutboxEvent = outbox.Event

type OutboxWriter interface {
	Save(ctx context.Context, event OutboxEvent) error
}
