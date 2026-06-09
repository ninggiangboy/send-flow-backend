package app

import (
	"errors"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/domain"
	deliverycontracts "github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/contracts"
	trackingcontracts "github.com/ninggiangboy/send-flow/backend/internal/modules/tracking/contracts"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/events"
)

var ErrUnsupportedEventType = errors.New("unsupported event type")
var ErrMalformedPayload = errors.New("malformed event payload")

type MappedEvent struct {
	Input IngestEmailEventFactInput
}

func MapEnvelopeToEvent(envelope events.Envelope) (*MappedEvent, error) {
	switch envelope.EventType {
	case deliverycontracts.EventDeliveryMessageQueuedV1:
		return mapMessageQueued(envelope)
	case deliverycontracts.EventDeliveryMessageAcceptedV1:
		return mapMessageAccepted(envelope)
	case deliverycontracts.EventDeliveryMessageDeliveredV1:
		return mapMessageDelivered(envelope)
	case deliverycontracts.EventDeliveryMessageBouncedV1:
		return mapMessageBounced(envelope)
	case deliverycontracts.EventDeliveryMessageComplainedV1:
		return mapMessageComplained(envelope)
	case deliverycontracts.EventDeliveryMessageRetryScheduledV1:
		return mapRetryScheduled(envelope)
	case trackingcontracts.EventEmailOpenedV1:
		return mapEmailOpened(envelope)
	case trackingcontracts.EventLinkClickedV1:
		return mapLinkClicked(envelope)
	case trackingcontracts.EventRecipientUnsubscribedV1:
		return mapRecipientUnsubscribed(envelope)
	default:
		return nil, ErrUnsupportedEventType
	}
}

func mapMessageQueued(envelope events.Envelope) (*MappedEvent, error) {
	var p deliverycontracts.MessageQueuedPayload
	if err := envelope.DecodePayload(&p); err != nil {
		return nil, errors.Join(ErrMalformedPayload, err)
	}

	return &MappedEvent{
		Input: IngestEmailEventFactInput{
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

func mapMessageAccepted(envelope events.Envelope) (*MappedEvent, error) {
	var p deliverycontracts.MessageAcceptedPayload
	if err := envelope.DecodePayload(&p); err != nil {
		return nil, errors.Join(ErrMalformedPayload, err)
	}

	occurredAt, err := parseTimestamp(p.AcceptedAt)
	if err != nil {
		return nil, errors.Join(ErrMalformedPayload, err)
	}

	return &MappedEvent{
		Input: IngestEmailEventFactInput{
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

func mapMessageDelivered(envelope events.Envelope) (*MappedEvent, error) {
	var p deliverycontracts.MessageDeliveredPayload
	if err := envelope.DecodePayload(&p); err != nil {
		return nil, errors.Join(ErrMalformedPayload, err)
	}

	occurredAt, err := parseTimestamp(p.OccurredAt)
	if err != nil {
		return nil, errors.Join(ErrMalformedPayload, err)
	}
	receivedAt, err := parseTimestamp(p.ReceivedAt)
	if err != nil {
		return nil, errors.Join(ErrMalformedPayload, err)
	}

	return &MappedEvent{
		Input: IngestEmailEventFactInput{
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

func mapMessageBounced(envelope events.Envelope) (*MappedEvent, error) {
	var p deliverycontracts.MessageBouncedPayload
	if err := envelope.DecodePayload(&p); err != nil {
		return nil, errors.Join(ErrMalformedPayload, err)
	}

	occurredAt, err := parseTimestamp(p.OccurredAt)
	if err != nil {
		return nil, errors.Join(ErrMalformedPayload, err)
	}
	receivedAt, err := parseTimestamp(p.ReceivedAt)
	if err != nil {
		return nil, errors.Join(ErrMalformedPayload, err)
	}

	return &MappedEvent{
		Input: IngestEmailEventFactInput{
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

func mapMessageComplained(envelope events.Envelope) (*MappedEvent, error) {
	var p deliverycontracts.MessageComplainedPayload
	if err := envelope.DecodePayload(&p); err != nil {
		return nil, errors.Join(ErrMalformedPayload, err)
	}

	occurredAt, err := parseTimestamp(p.OccurredAt)
	if err != nil {
		return nil, errors.Join(ErrMalformedPayload, err)
	}
	receivedAt, err := parseTimestamp(p.ReceivedAt)
	if err != nil {
		return nil, errors.Join(ErrMalformedPayload, err)
	}

	return &MappedEvent{
		Input: IngestEmailEventFactInput{
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

func mapRetryScheduled(envelope events.Envelope) (*MappedEvent, error) {
	var p deliverycontracts.MessageRetryScheduledPayload
	if err := envelope.DecodePayload(&p); err != nil {
		return nil, errors.Join(ErrMalformedPayload, err)
	}

	occurredAt, err := parseTimestamp(p.NextAttemptAt)
	if err != nil {
		return nil, errors.Join(ErrMalformedPayload, err)
	}

	return &MappedEvent{
		Input: IngestEmailEventFactInput{
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

func mapEmailOpened(envelope events.Envelope) (*MappedEvent, error) {
	var p trackingcontracts.EmailOpenedPayload
	if err := envelope.DecodePayload(&p); err != nil {
		return nil, errors.Join(ErrMalformedPayload, err)
	}

	occurredAt, err := parseTimestamp(p.OccurredAt)
	if err != nil {
		return nil, errors.Join(ErrMalformedPayload, err)
	}
	receivedAt, err := parseTimestamp(p.ReceivedAt)
	if err != nil {
		return nil, errors.Join(ErrMalformedPayload, err)
	}

	return &MappedEvent{
		Input: IngestEmailEventFactInput{
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

func mapLinkClicked(envelope events.Envelope) (*MappedEvent, error) {
	var p trackingcontracts.LinkClickedPayload
	if err := envelope.DecodePayload(&p); err != nil {
		return nil, errors.Join(ErrMalformedPayload, err)
	}

	occurredAt, err := parseTimestamp(p.OccurredAt)
	if err != nil {
		return nil, errors.Join(ErrMalformedPayload, err)
	}
	receivedAt, err := parseTimestamp(p.ReceivedAt)
	if err != nil {
		return nil, errors.Join(ErrMalformedPayload, err)
	}

	return &MappedEvent{
		Input: IngestEmailEventFactInput{
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

func mapRecipientUnsubscribed(envelope events.Envelope) (*MappedEvent, error) {
	var p trackingcontracts.RecipientUnsubscribedPayload
	if err := envelope.DecodePayload(&p); err != nil {
		return nil, errors.Join(ErrMalformedPayload, err)
	}

	occurredAt, err := parseTimestamp(p.OccurredAt)
	if err != nil {
		return nil, errors.Join(ErrMalformedPayload, err)
	}
	receivedAt, err := parseTimestamp(p.ReceivedAt)
	if err != nil {
		return nil, errors.Join(ErrMalformedPayload, err)
	}

	return &MappedEvent{
		Input: IngestEmailEventFactInput{
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

func parseTimestamp(s string) (time.Time, error) {
	return time.Parse(time.RFC3339, s)
}
