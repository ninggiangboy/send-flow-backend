package realtime

import (
	"fmt"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/notification/contracts"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/events"
)

func NormalizeEnvelope(env events.Envelope) (Event, bool, error) {
	switch env.EventType {
	case contracts.EventMessageQueuedV1:
		return normalizeQueued(env)
	case contracts.EventMessageSentV1:
		return normalizeSent(env)
	case contracts.EventMessageFailedV1:
		return normalizeFailed(env)
	default:
		return Event{}, false, nil
	}
}

func normalizeQueued(env events.Envelope) (Event, bool, error) {
	var p contracts.MessageQueuedPayload
	if err := env.DecodePayload(&p); err != nil {
		return Event{}, false, fmt.Errorf("decode queued payload: %w", err)
	}

	ev := Event{
		EventID:         env.EventID,
		EventType:       env.EventType,
		MessageID:       p.MessageID,
		WorkspaceID:     p.WorkspaceID,
		RecipientUserID: p.RecipientUserID,
		Type:            p.Type,
		Status:          "queued",
		OccurredAt:      env.OccurredAt,
	}
	if ev.MessageID == "" {
		ev.MessageID = env.AggregateID
	}
	if ev.WorkspaceID == "" {
		ev.WorkspaceID = env.WorkspaceID
	}
	return ev, true, nil
}

func normalizeSent(env events.Envelope) (Event, bool, error) {
	var p contracts.MessageSentPayload
	if err := env.DecodePayload(&p); err != nil {
		return Event{}, false, fmt.Errorf("decode sent payload: %w", err)
	}

	ev := Event{
		EventID:         env.EventID,
		EventType:       env.EventType,
		MessageID:       p.MessageID,
		WorkspaceID:     p.WorkspaceID,
		RecipientUserID: p.RecipientUserID,
		Type:            p.Type,
		Status:          "sent",
		AttemptNumber:   &p.AttemptNumber,
		Provider:        p.Provider,
		OccurredAt:      env.OccurredAt,
	}
	if ev.MessageID == "" {
		ev.MessageID = env.AggregateID
	}
	if ev.WorkspaceID == "" {
		ev.WorkspaceID = env.WorkspaceID
	}
	return ev, true, nil
}

func normalizeFailed(env events.Envelope) (Event, bool, error) {
	var p contracts.MessageFailedPayload
	if err := env.DecodePayload(&p); err != nil {
		return Event{}, false, fmt.Errorf("decode failed payload: %w", err)
	}

	final := p.FinalFailure
	ev := Event{
		EventID:         env.EventID,
		EventType:       env.EventType,
		MessageID:       p.MessageID,
		WorkspaceID:     p.WorkspaceID,
		RecipientUserID: p.RecipientUserID,
		Type:            p.Type,
		Status:          "failed",
		AttemptNumber:   &p.AttemptNumber,
		FinalFailure:    &final,
		OccurredAt:      env.OccurredAt,
	}
	if ev.MessageID == "" {
		ev.MessageID = env.AggregateID
	}
	if ev.WorkspaceID == "" {
		ev.WorkspaceID = env.WorkspaceID
	}
	return ev, true, nil
}
