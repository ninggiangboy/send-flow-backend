package usecase

import (
	"context"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/notification/contracts"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/notification/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/notification/ports"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/events"
)

type EventPublisher struct {
	outboxWriter ports.OutboxWriter
	idGen        func() (string, error)
}

func NewEventPublisher(outboxWriter ports.OutboxWriter, idGen func() (string, error)) *EventPublisher {
	return &EventPublisher{
		outboxWriter: outboxWriter,
		idGen:        idGen,
	}
}

func MustID(gen func() (string, error)) string {
	id, err := gen()
	if err != nil {
		panic(err)
	}
	return id
}

func (p *EventPublisher) PublishQueued(ctx context.Context, msg domain.NotificationMessage) error {
	eventID := MustID(p.idGen)
	ws := ""
	if msg.WorkspaceID != nil {
		ws = *msg.WorkspaceID
	}
	payload := contracts.MessageQueuedPayload{
		MessageID:      msg.ID,
		WorkspaceID:    ws,
		Type:           string(msg.Type),
		RecipientEmail: msg.RecipientEmail,
		Status:         string(msg.Status),
		CreatedAt:      msg.CreatedAt.Format(time.RFC3339),
	}
	envelope, err := events.NewEnvelope(events.NewEnvelopeOptions{
		EventID:       eventID,
		EventType:     contracts.EventMessageQueuedV1,
		EventVersion:  1,
		AggregateType: "notification_message",
		AggregateID:   msg.ID,
		WorkspaceID:   ws,
		OccurredAt:    msg.CreatedAt,
	}, payload)
	if err != nil {
		return err
	}
	payloadBytes, err := events.Marshal(envelope)
	if err != nil {
		return err
	}
	return p.outboxWriter.Save(ctx, ports.OutboxEvent{
		ID:            eventID,
		AggregateType: "notification_message",
		AggregateID:   msg.ID,
		EventType:     contracts.EventMessageQueuedV1,
		Payload:       payloadBytes,
		WorkspaceID:   ws,
		OccurredAt:    msg.CreatedAt,
	})
}

func (p *EventPublisher) PublishSent(ctx context.Context, msg domain.NotificationMessage, attemptNumber int, provider string) error {
	eventID := MustID(p.idGen)
	ws := ""
	if msg.WorkspaceID != nil {
		ws = *msg.WorkspaceID
	}
	now := time.Now().UTC()
	payload := contracts.MessageSentPayload{
		MessageID:      msg.ID,
		WorkspaceID:    ws,
		Type:           string(msg.Type),
		RecipientEmail: msg.RecipientEmail,
		AttemptNumber:  attemptNumber,
		Provider:       provider,
		SentAt:         now.Format(time.RFC3339),
	}
	envelope, err := events.NewEnvelope(events.NewEnvelopeOptions{
		EventID:       eventID,
		EventType:     contracts.EventMessageSentV1,
		EventVersion:  1,
		AggregateType: "notification_message",
		AggregateID:   msg.ID,
		WorkspaceID:   ws,
		OccurredAt:    now,
	}, payload)
	if err != nil {
		return err
	}
	payloadBytes, err := events.Marshal(envelope)
	if err != nil {
		return err
	}
	return p.outboxWriter.Save(ctx, ports.OutboxEvent{
		ID:            eventID,
		AggregateType: "notification_message",
		AggregateID:   msg.ID,
		EventType:     contracts.EventMessageSentV1,
		Payload:       payloadBytes,
		WorkspaceID:   ws,
		OccurredAt:    now,
	})
}

func (p *EventPublisher) PublishFailed(ctx context.Context, msg domain.NotificationMessage, attemptNumber int, errorMessage string, finalFailure bool) error {
	eventID := MustID(p.idGen)
	ws := ""
	if msg.WorkspaceID != nil {
		ws = *msg.WorkspaceID
	}
	now := time.Now().UTC()
	payload := contracts.MessageFailedPayload{
		MessageID:      msg.ID,
		WorkspaceID:    ws,
		Type:           string(msg.Type),
		RecipientEmail: msg.RecipientEmail,
		AttemptNumber:  attemptNumber,
		ErrorMessage:   errorMessage,
		FinalFailure:   finalFailure,
	}
	envelope, err := events.NewEnvelope(events.NewEnvelopeOptions{
		EventID:       eventID,
		EventType:     contracts.EventMessageFailedV1,
		EventVersion:  1,
		AggregateType: "notification_message",
		AggregateID:   msg.ID,
		WorkspaceID:   ws,
		OccurredAt:    now,
	}, payload)
	if err != nil {
		return err
	}
	payloadBytes, err := events.Marshal(envelope)
	if err != nil {
		return err
	}
	return p.outboxWriter.Save(ctx, ports.OutboxEvent{
		ID:            eventID,
		EventType:     contracts.EventMessageFailedV1,
		AggregateType: "notification_message",
		AggregateID:   msg.ID,
		Payload:       payloadBytes,
		WorkspaceID:   ws,
		OccurredAt:    now,
	})
}
