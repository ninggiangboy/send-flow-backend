package analyticsmappers

import (
	"errors"

	analyticsapp "github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/app"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/domain"
	trackingcontracts "github.com/ninggiangboy/send-flow/backend/internal/modules/tracking/contracts"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/events"
)

func RegisterAll(r *analyticsapp.MapperRegistry) {
	r.Register(trackingcontracts.EventEmailOpenedV1, MapEmailOpened)
	r.Register(trackingcontracts.EventLinkClickedV1, MapLinkClicked)
	r.Register(trackingcontracts.EventRecipientUnsubscribedV1, MapRecipientUnsubscribed)
}

func MapEmailOpened(envelope events.Envelope) (*analyticsapp.MappedEvent, error) {
	var p trackingcontracts.EmailOpenedPayload
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
			CanonicalType:     domain.EventTypeOpened,
			OccurredAt:        occurredAt,
			ReceivedAt:        receivedAt,
		},
	}, nil
}

func MapLinkClicked(envelope events.Envelope) (*analyticsapp.MappedEvent, error) {
	var p trackingcontracts.LinkClickedPayload
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
			CanonicalType:     domain.EventTypeClicked,
			OccurredAt:        occurredAt,
			ReceivedAt:        receivedAt,
		},
	}, nil
}

func MapRecipientUnsubscribed(envelope events.Envelope) (*analyticsapp.MappedEvent, error) {
	var p trackingcontracts.RecipientUnsubscribedPayload
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
			SourceEventID:   envelope.EventID,
			SourceEventType: envelope.EventType,
			WorkspaceID:     p.WorkspaceID,
			CampaignID:      p.CampaignID,
			MessageID:       p.MessageID,
			CanonicalType:   domain.EventTypeUnsubscribed,
			OccurredAt:      occurredAt,
			ReceivedAt:      receivedAt,
		},
	}, nil
}
