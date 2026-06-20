package event

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/tracking/contracts"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/tracking/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/tracking/ports"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/events"
)

type ClickOptions struct {
	LinkReadRepo    ports.TrackingLinkReadRepository
	EventReadRepo   ports.TrackingEventReadRepository
	EventWriteRepo  ports.TrackingEventWriteRepository
	MessageResolver ports.DeliveryMessageResolver
	OutboxWriter    ports.OutboxWriter
	TxManager       ports.TransactionManager
	IDGen           func() (string, error)
	Logger          *slog.Logger
}

type ClickHandler struct {
	linkReadRepo    ports.TrackingLinkReadRepository
	eventReadRepo   ports.TrackingEventReadRepository
	eventWriteRepo  ports.TrackingEventWriteRepository
	messageResolver ports.DeliveryMessageResolver
	outboxWriter    ports.OutboxWriter
	txManager       ports.TransactionManager
	idGen           func() (string, error)
	log             *slog.Logger
}

func NewClick(opts ClickOptions) *ClickHandler {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	return &ClickHandler{
		linkReadRepo:    opts.LinkReadRepo,
		eventReadRepo:   opts.EventReadRepo,
		eventWriteRepo:  opts.EventWriteRepo,
		messageResolver: opts.MessageResolver,
		outboxWriter:    opts.OutboxWriter,
		txManager:       opts.TxManager,
		idGen:           opts.IDGen,
		log:             opts.Logger.With("usecase", "record_click"),
	}
}

type ClickInput struct {
	TrackingID        string
	WorkspaceID       string
	MessageID         string
	Provider          string
	ProviderMessageID string
	ProviderEventID   string
	NormalizedEventID string
	Source            string
	SourceEventID     string
	OccurredAt        time.Time
	ReceivedAt        time.Time
}

type ClickResult struct {
	TrackingEventID string
	DestinationURL  string
}

func (h *ClickHandler) Execute(ctx context.Context, input ClickInput) (*ClickResult, error) {
	log := h.log.With("source", input.Source)

	now := time.Now().UTC()
	if input.ReceivedAt.IsZero() {
		input.ReceivedAt = now
	}
	if input.OccurredAt.IsZero() {
		input.OccurredAt = now
	}

	var resolvedWorkspaceID, resolvedMessageID, resolvedCampaignID, resolvedProvider, resolvedProviderMessageID, destinationURL string

	if input.TrackingID != "" {
		link, err := h.linkReadRepo.FindByID(ctx, input.TrackingID)
		if err != nil {
			if errors.Is(err, domain.ErrTrackingLinkNotFound) {
				log.Info("tracking link not found for click", "tracking_id", input.TrackingID)
				return &ClickResult{}, nil
			}
			log.Error("failed to find tracking link", "tracking_id", input.TrackingID, "error", err)
			return nil, err
		}
		if link.ExpiresAt != nil && now.After(*link.ExpiresAt) {
			log.Info("tracking link expired for click", "tracking_id", input.TrackingID)
			return &ClickResult{}, nil
		}
		resolvedWorkspaceID = link.WorkspaceID
		resolvedMessageID = link.MessageID
		destinationURL = link.DestinationURL
	}

	if resolvedMessageID == "" && input.WorkspaceID != "" && input.MessageID != "" {
		msgID, wsID, campaignID, provider, providerMsgID, _, err := h.messageResolver.FindByID(ctx, input.WorkspaceID, input.MessageID)
		if err != nil {
			log.Info("message not found for click event", "workspace_id", input.WorkspaceID, "message_id", input.MessageID)
			return &ClickResult{}, nil
		}
		resolvedWorkspaceID = wsID
		resolvedMessageID = msgID
		resolvedCampaignID = campaignID
		resolvedProvider = provider
		resolvedProviderMessageID = providerMsgID
	}

	if resolvedMessageID == "" && input.Provider != "" && input.ProviderMessageID != "" {
		msgID, wsID, campaignID, err := h.messageResolver.FindByProviderMessageID(ctx, input.Provider, input.ProviderMessageID)
		if err != nil {
			log.Info("message not found by provider message id for click event", "provider", input.Provider, "provider_message_id", input.ProviderMessageID)
			return &ClickResult{}, nil
		}
		resolvedWorkspaceID = wsID
		resolvedMessageID = msgID
		resolvedCampaignID = campaignID
		resolvedProvider = input.Provider
		resolvedProviderMessageID = input.ProviderMessageID
	}

	if resolvedMessageID == "" {
		log.Info("no message resolved for click event, skipping")
		return &ClickResult{}, nil
	}

	if input.Provider != "" {
		resolvedProvider = input.Provider
	}
	if input.ProviderMessageID != "" {
		resolvedProviderMessageID = input.ProviderMessageID
	}

	source := input.Source
	if source == "" {
		source = domain.SourceHTTP
	}

	var trackingEventResult *ClickResult

	if err := h.txManager.WithinTx(ctx, func(txCtx context.Context) error {
		if input.SourceEventID != "" {
			existing, err := h.eventReadRepo.FindBySourceEvent(txCtx, source, input.SourceEventID, domain.EventTypeClick)
			if err != nil {
				log.Error("failed to check existing click event", "error", err)
				return err
			}
			if existing != nil {
				log.Info("duplicate click event, skipping", "tracking_event_id", existing.ID)
				trackingEventResult = &ClickResult{TrackingEventID: existing.ID, DestinationURL: destinationURL}
				return nil
			}
		}

		eventID, err := h.idGen()
		if err != nil {
			log.Error("failed to generate event id", "error", err)
			return err
		}

		event := domain.TrackingEvent{
			ID:                eventID,
			WorkspaceID:       resolvedWorkspaceID,
			MessageID:         resolvedMessageID,
			TrackingLinkID:    input.TrackingID,
			EventType:         domain.EventTypeClick,
			Source:            source,
			SourceEventID:     input.SourceEventID,
			Provider:          resolvedProvider,
			ProviderEventID:   input.ProviderEventID,
			ProviderMessageID: resolvedProviderMessageID,
			OccurredAt:        input.OccurredAt,
			ReceivedAt:        input.ReceivedAt,
			Metadata:          map[string]any{},
			CreatedAt:         now,
		}

		if err := h.eventWriteRepo.Create(txCtx, event); err != nil {
			if errors.Is(err, domain.ErrTrackingEventConflict) {
				log.Info("click event conflict (idempotent), skipping")
				return nil
			}
			log.Error("failed to create click tracking event", "error", err)
			return err
		}

		outboxEventID, err := h.idGen()
		if err != nil {
			log.Error("failed to generate outbox event id", "error", err)
			return err
		}

		payload := contracts.LinkClickedPayload{
			TrackingEventID:   eventID,
			TrackingLinkID:    input.TrackingID,
			WorkspaceID:       resolvedWorkspaceID,
			MessageID:         resolvedMessageID,
			CampaignID:        resolvedCampaignID,
			DestinationURL:    destinationURL,
			Provider:          resolvedProvider,
			ProviderMessageID: resolvedProviderMessageID,
			ProviderEventID:   input.ProviderEventID,
			NormalizedEventID: input.NormalizedEventID,
			OccurredAt:        input.OccurredAt.Format(time.RFC3339),
			ReceivedAt:        input.ReceivedAt.Format(time.RFC3339),
		}

		envelope, err := events.NewEnvelope(events.NewEnvelopeOptions{
			EventID:       outboxEventID,
			EventType:     contracts.EventLinkClickedV1,
			EventVersion:  1,
			AggregateType: contracts.AggregateTrackingEvent,
			AggregateID:   eventID,
			WorkspaceID:   resolvedWorkspaceID,
			OccurredAt:    now,
		}, payload)
		if err != nil {
			log.Error("failed to create outbox envelope", "error", err)
			return err
		}

		payloadBytes, err := events.Marshal(envelope)
		if err != nil {
			log.Error("failed to marshal outbox event", "error", err)
			return err
		}

		if err := h.outboxWriter.Save(txCtx, ports.OutboxEvent{
			ID:            outboxEventID,
			AggregateType: contracts.AggregateTrackingEvent,
			AggregateID:   eventID,
			EventType:     contracts.EventLinkClickedV1,
			Payload:       payloadBytes,
			WorkspaceID:   resolvedWorkspaceID,
			OccurredAt:    now,
		}); err != nil {
			log.Error("failed to save outbox event", "error", err)
			return err
		}

		trackingEventResult = &ClickResult{TrackingEventID: eventID, DestinationURL: destinationURL}
		return nil
	}); err != nil {
		log.Error("record click transaction failed", "error", err)
		return nil, err
	}

	log.Info("click event recorded",
		"tracking_event_id", trackingEventResult.TrackingEventID,
		"workspace_id", resolvedWorkspaceID,
		"message_id", resolvedMessageID,
	)
	return trackingEventResult, nil
}
