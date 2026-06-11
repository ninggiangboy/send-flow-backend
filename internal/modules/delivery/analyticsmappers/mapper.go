package analyticsmappers

import (
	"errors"

	analyticsapp "github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/app"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/domain"
	deliverycontracts "github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/contracts"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/events"
)

func RegisterAll(r *analyticsapp.MapperRegistry) {
	r.Register(deliverycontracts.EventDeliveryMessageQueuedV1, MapMessageQueued)
	r.Register(deliverycontracts.EventDeliveryMessageAcceptedV1, MapMessageAccepted)
	r.Register(deliverycontracts.EventDeliveryMessageDeliveredV1, MapMessageDelivered)
	r.Register(deliverycontracts.EventDeliveryMessageBouncedV1, MapMessageBounced)
	r.Register(deliverycontracts.EventDeliveryMessageComplainedV1, MapMessageComplained)
	r.Register(deliverycontracts.EventDeliveryMessageRetryScheduledV1, MapRetryScheduled)
}

func MapMessageQueued(envelope events.Envelope) (*analyticsapp.MappedEvent, error) {
	var p deliverycontracts.MessageQueuedPayload
	if err := envelope.DecodePayload(&p); err != nil {
		return nil, errors.Join(analyticsapp.ErrMalformedPayload, err)
	}

	return &analyticsapp.MappedEvent{
		Input: analyticsapp.IngestEmailEventFactInput{
			SourceEventID:   envelope.EventID,
			SourceEventType: envelope.EventType,
			WorkspaceID:     p.WorkspaceID,
			CampaignID:      p.CampaignID,
			MessageID:       p.MessageID,
			CanonicalType:   domain.EventTypeQueued,
			OccurredAt:      envelope.OccurredAt,
			ReceivedAt:      envelope.OccurredAt,
		},
	}, nil
}

func MapMessageAccepted(envelope events.Envelope) (*analyticsapp.MappedEvent, error) {
	var p deliverycontracts.MessageAcceptedPayload
	if err := envelope.DecodePayload(&p); err != nil {
		return nil, errors.Join(analyticsapp.ErrMalformedPayload, err)
	}

	occurredAt, err := analyticsapp.ParseTimestamp(p.AcceptedAt)
	if err != nil {
		return nil, errors.Join(analyticsapp.ErrMalformedPayload, err)
	}

	return &analyticsapp.MappedEvent{
		Input: analyticsapp.IngestEmailEventFactInput{
			SourceEventID:     envelope.EventID,
			SourceEventType:   envelope.EventType,
			WorkspaceID:       p.WorkspaceID,
			CampaignID:        p.CampaignID,
			MessageID:         p.MessageID,
			Provider:          p.Provider,
			ProviderMessageID: p.ProviderMessageID,
			CanonicalType:     domain.EventTypeAccepted,
			OccurredAt:        occurredAt,
			ReceivedAt:        envelope.OccurredAt,
		},
	}, nil
}

func MapMessageDelivered(envelope events.Envelope) (*analyticsapp.MappedEvent, error) {
	var p deliverycontracts.MessageDeliveredPayload
	if err := envelope.DecodePayload(&p); err != nil {
		return nil, errors.Join(analyticsapp.ErrMalformedPayload, err)
	}

	occurredAt, err := analyticsapp.ParseTimestamp(p.OccurredAt)
	if err != nil {
		return nil, errors.Join(analyticsapp.ErrMalformedPayload, err)
	}
	receivedAt, err := analyticsapp.ParseTimestamp(p.ReceivedAt)
	if err != nil {
		return nil, errors.Join(analyticsapp.ErrMalformedPayload, err)
	}

	return &analyticsapp.MappedEvent{
		Input: analyticsapp.IngestEmailEventFactInput{
			SourceEventID:     envelope.EventID,
			SourceEventType:   envelope.EventType,
			WorkspaceID:       p.WorkspaceID,
			CampaignID:        p.CampaignID,
			MessageID:         p.MessageID,
			Provider:          p.Provider,
			ProviderMessageID: p.ProviderMessageID,
			ProviderEventID:   p.ProviderEventID,
			CanonicalType:     domain.EventTypeDelivered,
			OccurredAt:        occurredAt,
			ReceivedAt:        receivedAt,
		},
	}, nil
}

func MapMessageBounced(envelope events.Envelope) (*analyticsapp.MappedEvent, error) {
	var p deliverycontracts.MessageBouncedPayload
	if err := envelope.DecodePayload(&p); err != nil {
		return nil, errors.Join(analyticsapp.ErrMalformedPayload, err)
	}

	occurredAt, err := analyticsapp.ParseTimestamp(p.OccurredAt)
	if err != nil {
		return nil, errors.Join(analyticsapp.ErrMalformedPayload, err)
	}
	receivedAt, err := analyticsapp.ParseTimestamp(p.ReceivedAt)
	if err != nil {
		return nil, errors.Join(analyticsapp.ErrMalformedPayload, err)
	}

	return &analyticsapp.MappedEvent{
		Input: analyticsapp.IngestEmailEventFactInput{
			SourceEventID:     envelope.EventID,
			SourceEventType:   envelope.EventType,
			WorkspaceID:       p.WorkspaceID,
			CampaignID:        p.CampaignID,
			MessageID:         p.MessageID,
			Provider:          p.Provider,
			ProviderMessageID: p.ProviderMessageID,
			ProviderEventID:   p.ProviderEventID,
			CanonicalType:     domain.EventTypeBounced,
			OccurredAt:        occurredAt,
			ReceivedAt:        receivedAt,
		},
	}, nil
}

func MapMessageComplained(envelope events.Envelope) (*analyticsapp.MappedEvent, error) {
	var p deliverycontracts.MessageComplainedPayload
	if err := envelope.DecodePayload(&p); err != nil {
		return nil, errors.Join(analyticsapp.ErrMalformedPayload, err)
	}

	occurredAt, err := analyticsapp.ParseTimestamp(p.OccurredAt)
	if err != nil {
		return nil, errors.Join(analyticsapp.ErrMalformedPayload, err)
	}
	receivedAt, err := analyticsapp.ParseTimestamp(p.ReceivedAt)
	if err != nil {
		return nil, errors.Join(analyticsapp.ErrMalformedPayload, err)
	}

	return &analyticsapp.MappedEvent{
		Input: analyticsapp.IngestEmailEventFactInput{
			SourceEventID:     envelope.EventID,
			SourceEventType:   envelope.EventType,
			WorkspaceID:       p.WorkspaceID,
			CampaignID:        p.CampaignID,
			MessageID:         p.MessageID,
			Provider:          p.Provider,
			ProviderMessageID: p.ProviderMessageID,
			ProviderEventID:   p.ProviderEventID,
			CanonicalType:     domain.EventTypeComplained,
			OccurredAt:        occurredAt,
			ReceivedAt:        receivedAt,
		},
	}, nil
}

func MapRetryScheduled(envelope events.Envelope) (*analyticsapp.MappedEvent, error) {
	var p deliverycontracts.MessageRetryScheduledPayload
	if err := envelope.DecodePayload(&p); err != nil {
		return nil, errors.Join(analyticsapp.ErrMalformedPayload, err)
	}

	occurredAt, err := analyticsapp.ParseTimestamp(p.NextAttemptAt)
	if err != nil {
		return nil, errors.Join(analyticsapp.ErrMalformedPayload, err)
	}

	return &analyticsapp.MappedEvent{
		Input: analyticsapp.IngestEmailEventFactInput{
			SourceEventID:   envelope.EventID,
			SourceEventType: envelope.EventType,
			WorkspaceID:     p.WorkspaceID,
			MessageID:       p.MessageID,
			CanonicalType:   domain.EventTypeRetryScheduled,
			OccurredAt:      occurredAt,
			ReceivedAt:      envelope.OccurredAt,
		},
	}, nil
}
