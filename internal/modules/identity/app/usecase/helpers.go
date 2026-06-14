package usecase

import (
	"context"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/ports"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/events"
)

func EmitEvent(ctx context.Context, outbox ports.OutboxWriter, idGen ports.IDGenerator, eventType, aggregateType, aggregateID, workspaceID string, payload any, now time.Time) error {
	if outbox == nil {
		return nil
	}
	eventID, err := idGen.New()
	if err != nil {
		return err
	}
	env, err := events.NewEnvelope(events.NewEnvelopeOptions{
		EventID:       eventID,
		EventType:     eventType,
		EventVersion:  1,
		AggregateType: aggregateType,
		AggregateID:   aggregateID,
		WorkspaceID:   workspaceID,
		OccurredAt:    now,
	}, payload)
	if err != nil {
		return err
	}
	payloadBytes, err := events.Marshal(env)
	if err != nil {
		return err
	}
	return outbox.Save(ctx, ports.OutboxEvent{
		ID:            eventID,
		AggregateType: aggregateType,
		AggregateID:   aggregateID,
		EventType:     eventType,
		Payload:       payloadBytes,
		WorkspaceID:   workspaceID,
		OccurredAt:    now,
	})
}
