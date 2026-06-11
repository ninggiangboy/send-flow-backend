package app

import (
	"context"
	"errors"
	"log/slog"
	"time"

	suppressioncontracts "github.com/ninggiangboy/send-flow/backend/internal/modules/suppression/contracts"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/tracking/app/unsubscribetoken"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/tracking/contracts"
	trackingdomain "github.com/ninggiangboy/send-flow/backend/internal/modules/tracking/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/tracking/ports"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/events"
	platformerrors "github.com/ninggiangboy/send-flow/backend/internal/platform/retryable"
)

type RecipientSuppressor interface {
	SuppressFromSignal(ctx context.Context, input SuppressFromSignalInput) (*SuppressFromSignalResult, error)
}

type SuppressFromSignalInput struct {
	WorkspaceID     string
	Email           string
	EmailNormalized string
	Scope           string
	Reason          string
	Source          string
	SourceEventID   string
	Note            string
	Now             time.Time
}

type SuppressFromSignalResult struct {
	EntryID string
	Created bool
}

type Options struct {
	LinkReadRepo        ports.TrackingLinkReadRepository
	LinkWriteRepo       ports.TrackingLinkWriteRepository
	EventReadRepo       ports.TrackingEventReadRepository
	EventWriteRepo      ports.TrackingEventWriteRepository
	MessageResolver     ports.DeliveryMessageResolver
	RecipientSuppressor RecipientSuppressor
	OutboxWriter        ports.OutboxWriter
	TxManager           ports.TransactionManager
	IDGen               func() (string, error)
	Logger              *slog.Logger
	TokenSigner         *unsubscribetoken.Signer
}

type Service struct {
	linkReadRepo        ports.TrackingLinkReadRepository
	linkWriteRepo       ports.TrackingLinkWriteRepository
	eventReadRepo       ports.TrackingEventReadRepository
	eventWriteRepo      ports.TrackingEventWriteRepository
	messageResolver     ports.DeliveryMessageResolver
	recipientSuppressor RecipientSuppressor
	outboxWriter        ports.OutboxWriter
	txManager           ports.TransactionManager
	idGen               func() (string, error)
	log                 *slog.Logger
	tokenSigner         *unsubscribetoken.Signer
}

func NewService(opts Options) *Service {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.IDGen == nil {
		opts.IDGen = func() (string, error) { return "", nil }
	}
	return &Service{
		linkReadRepo:        opts.LinkReadRepo,
		linkWriteRepo:       opts.LinkWriteRepo,
		eventReadRepo:       opts.EventReadRepo,
		eventWriteRepo:      opts.EventWriteRepo,
		messageResolver:     opts.MessageResolver,
		recipientSuppressor: opts.RecipientSuppressor,
		outboxWriter:        opts.OutboxWriter,
		txManager:           opts.TxManager,
		idGen:               opts.IDGen,
		log:                 opts.Logger.With("module", "tracking"),
		tokenSigner:         opts.TokenSigner,
	}
}

type CreateTrackingLinkInput struct {
	WorkspaceID    string
	MessageID      string
	DestinationURL string
	LinkType       string
	Metadata       map[string]any
	Now            time.Time
	ExpiresAt      *time.Time
}

func (s *Service) CreateTrackingLink(ctx context.Context, input CreateTrackingLinkInput) (*trackingdomain.TrackingLink, error) {
	log := s.log.With("usecase", "create_tracking_link", "workspace_id", input.WorkspaceID)

	if !trackingdomain.ValidLinkType(input.LinkType) {
		return nil, &platformerrors.NonRetryableError{Err: trackingdomain.ErrTrackingEventInvalid}
	}
	if input.DestinationURL != "" {
		if err := trackingdomain.ValidateDestinationURL(input.DestinationURL); err != nil {
			return nil, &platformerrors.NonRetryableError{Err: err}
		}
	}

	id, err := s.idGen()
	if err != nil {
		log.Error("failed to generate id", "error", err)
		return nil, err
	}

	link := trackingdomain.TrackingLink{
		ID:             id,
		WorkspaceID:    input.WorkspaceID,
		MessageID:      input.MessageID,
		DestinationURL: input.DestinationURL,
		LinkType:       input.LinkType,
		Metadata:       input.Metadata,
		CreatedAt:      input.Now,
		ExpiresAt:      input.ExpiresAt,
	}

	if link.Metadata == nil {
		link.Metadata = map[string]any{}
	}

	if err := s.linkWriteRepo.Create(ctx, link); err != nil {
		log.Error("failed to create tracking link", "error", err)
		return nil, err
	}

	log.Info("tracking link created", "tracking_link_id", id, "link_type", input.LinkType)
	return &link, nil
}

type RecordOpenInput struct {
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

type RecordOpenResult struct {
	TrackingEventID string
}

func (s *Service) RecordOpen(ctx context.Context, input RecordOpenInput) (*RecordOpenResult, error) {
	log := s.log.With("usecase", "record_open", "source", input.Source)

	now := time.Now().UTC()
	if input.ReceivedAt.IsZero() {
		input.ReceivedAt = now
	}
	if input.OccurredAt.IsZero() {
		input.OccurredAt = now
	}

	var resolvedWorkspaceID, resolvedMessageID, resolvedCampaignID, resolvedProvider, resolvedProviderMessageID string

	if input.TrackingID != "" {
		link, err := s.linkReadRepo.FindByID(ctx, input.TrackingID)
		if err != nil {
			if errors.Is(err, trackingdomain.ErrTrackingLinkNotFound) {
				log.Info("tracking link not found for open pixel", "tracking_id", input.TrackingID)
				return &RecordOpenResult{}, nil
			}
			log.Error("failed to find tracking link", "tracking_id", input.TrackingID, "error", err)
			return nil, err
		}
		if link.ExpiresAt != nil && now.After(*link.ExpiresAt) {
			log.Info("tracking link expired for open pixel", "tracking_id", input.TrackingID)
			return &RecordOpenResult{}, nil
		}
		resolvedWorkspaceID = link.WorkspaceID
		resolvedMessageID = link.MessageID
	}

	if resolvedMessageID == "" && input.WorkspaceID != "" && input.MessageID != "" {
		msgID, wsID, campaignID, provider, providerMsgID, _, err := s.messageResolver.FindByID(ctx, input.WorkspaceID, input.MessageID)
		if err != nil {
			log.Info("message not found for open event", "workspace_id", input.WorkspaceID, "message_id", input.MessageID)
			return &RecordOpenResult{}, nil
		}
		resolvedWorkspaceID = wsID
		resolvedMessageID = msgID
		resolvedCampaignID = campaignID
		resolvedProvider = provider
		resolvedProviderMessageID = providerMsgID
	}

	if resolvedMessageID == "" && input.Provider != "" && input.ProviderMessageID != "" {
		msgID, wsID, campaignID, err := s.messageResolver.FindByProviderMessageID(ctx, input.Provider, input.ProviderMessageID)
		if err != nil {
			log.Info("message not found by provider message id for open event", "provider", input.Provider, "provider_message_id", input.ProviderMessageID)
			return &RecordOpenResult{}, nil
		}
		resolvedWorkspaceID = wsID
		resolvedMessageID = msgID
		resolvedCampaignID = campaignID
		resolvedProvider = input.Provider
		resolvedProviderMessageID = input.ProviderMessageID
	}

	if resolvedMessageID == "" {
		log.Info("no message resolved for open event, skipping")
		return &RecordOpenResult{}, nil
	}

	if input.Provider != "" {
		resolvedProvider = input.Provider
	}
	if input.ProviderMessageID != "" {
		resolvedProviderMessageID = input.ProviderMessageID
	}

	source := input.Source
	if source == "" {
		source = trackingdomain.SourceHTTP
	}

	var trackingEventResult *RecordOpenResult

	if err := s.txManager.WithinTx(ctx, func(txCtx context.Context) error {
		if input.SourceEventID != "" {
			existing, err := s.eventReadRepo.FindBySourceEvent(txCtx, source, input.SourceEventID, trackingdomain.EventTypeOpen)
			if err != nil {
				log.Error("failed to check existing open event", "error", err)
				return err
			}
			if existing != nil {
				log.Info("duplicate open event, skipping", "tracking_event_id", existing.ID)
				trackingEventResult = &RecordOpenResult{TrackingEventID: existing.ID}
				return nil
			}
		}

		eventID, err := s.idGen()
		if err != nil {
			log.Error("failed to generate event id", "error", err)
			return err
		}

		event := trackingdomain.TrackingEvent{
			ID:                eventID,
			WorkspaceID:       resolvedWorkspaceID,
			MessageID:         resolvedMessageID,
			TrackingLinkID:    input.TrackingID,
			EventType:         trackingdomain.EventTypeOpen,
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

		if err := s.eventWriteRepo.Create(txCtx, event); err != nil {
			if errors.Is(err, trackingdomain.ErrTrackingEventConflict) {
				log.Info("open event conflict (idempotent), skipping")
				return nil
			}
			log.Error("failed to create open tracking event", "error", err)
			return err
		}

		outboxEventID, err := s.idGen()
		if err != nil {
			log.Error("failed to generate outbox event id", "error", err)
			return err
		}

		payload := contracts.EmailOpenedPayload{
			TrackingEventID:   eventID,
			WorkspaceID:       resolvedWorkspaceID,
			MessageID:         resolvedMessageID,
			CampaignID:        resolvedCampaignID,
			Provider:          resolvedProvider,
			ProviderMessageID: resolvedProviderMessageID,
			ProviderEventID:   input.ProviderEventID,
			NormalizedEventID: input.NormalizedEventID,
			OccurredAt:        input.OccurredAt.Format(time.RFC3339),
			ReceivedAt:        input.ReceivedAt.Format(time.RFC3339),
		}

		envelope, err := events.NewEnvelope(events.NewEnvelopeOptions{
			EventID:       outboxEventID,
			EventType:     contracts.EventEmailOpenedV1,
			EventVersion:  1,
			AggregateType: "tracking_event",
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

		if err := s.outboxWriter.Save(txCtx, ports.OutboxEvent{
			ID:            outboxEventID,
			AggregateType: "tracking_event",
			AggregateID:   eventID,
			EventType:     contracts.EventEmailOpenedV1,
			Payload:       payloadBytes,
			WorkspaceID:   resolvedWorkspaceID,
			OccurredAt:    now,
		}); err != nil {
			log.Error("failed to save outbox event", "error", err)
			return err
		}

		trackingEventResult = &RecordOpenResult{TrackingEventID: eventID}
		return nil
	}); err != nil {
		log.Error("record open transaction failed", "error", err)
		return nil, err
	}

	log.Info("open event recorded",
		"tracking_event_id", trackingEventResult.TrackingEventID,
		"workspace_id", resolvedWorkspaceID,
		"message_id", resolvedMessageID,
	)
	return trackingEventResult, nil
}

type RecordClickInput struct {
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

type RecordClickResult struct {
	TrackingEventID string
	DestinationURL  string
}

func (s *Service) RecordClick(ctx context.Context, input RecordClickInput) (*RecordClickResult, error) {
	log := s.log.With("usecase", "record_click", "source", input.Source)

	now := time.Now().UTC()
	if input.ReceivedAt.IsZero() {
		input.ReceivedAt = now
	}
	if input.OccurredAt.IsZero() {
		input.OccurredAt = now
	}

	var resolvedWorkspaceID, resolvedMessageID, resolvedCampaignID, resolvedProvider, resolvedProviderMessageID, destinationURL string

	if input.TrackingID != "" {
		link, err := s.linkReadRepo.FindByID(ctx, input.TrackingID)
		if err != nil {
			if errors.Is(err, trackingdomain.ErrTrackingLinkNotFound) {
				log.Info("tracking link not found for click", "tracking_id", input.TrackingID)
				return &RecordClickResult{}, nil
			}
			log.Error("failed to find tracking link", "tracking_id", input.TrackingID, "error", err)
			return nil, err
		}
		if link.ExpiresAt != nil && now.After(*link.ExpiresAt) {
			log.Info("tracking link expired for click", "tracking_id", input.TrackingID)
			return &RecordClickResult{}, nil
		}
		resolvedWorkspaceID = link.WorkspaceID
		resolvedMessageID = link.MessageID
		destinationURL = link.DestinationURL
	}

	if resolvedMessageID == "" && input.WorkspaceID != "" && input.MessageID != "" {
		msgID, wsID, campaignID, provider, providerMsgID, _, err := s.messageResolver.FindByID(ctx, input.WorkspaceID, input.MessageID)
		if err != nil {
			log.Info("message not found for click event", "workspace_id", input.WorkspaceID, "message_id", input.MessageID)
			return &RecordClickResult{}, nil
		}
		resolvedWorkspaceID = wsID
		resolvedMessageID = msgID
		resolvedCampaignID = campaignID
		resolvedProvider = provider
		resolvedProviderMessageID = providerMsgID
	}

	if resolvedMessageID == "" && input.Provider != "" && input.ProviderMessageID != "" {
		msgID, wsID, campaignID, err := s.messageResolver.FindByProviderMessageID(ctx, input.Provider, input.ProviderMessageID)
		if err != nil {
			log.Info("message not found by provider message id for click event", "provider", input.Provider, "provider_message_id", input.ProviderMessageID)
			return &RecordClickResult{}, nil
		}
		resolvedWorkspaceID = wsID
		resolvedMessageID = msgID
		resolvedCampaignID = campaignID
		resolvedProvider = input.Provider
		resolvedProviderMessageID = input.ProviderMessageID
	}

	if resolvedMessageID == "" {
		log.Info("no message resolved for click event, skipping")
		return &RecordClickResult{}, nil
	}

	if input.Provider != "" {
		resolvedProvider = input.Provider
	}
	if input.ProviderMessageID != "" {
		resolvedProviderMessageID = input.ProviderMessageID
	}

	source := input.Source
	if source == "" {
		source = trackingdomain.SourceHTTP
	}

	var trackingEventResult *RecordClickResult

	if err := s.txManager.WithinTx(ctx, func(txCtx context.Context) error {
		if input.SourceEventID != "" {
			existing, err := s.eventReadRepo.FindBySourceEvent(txCtx, source, input.SourceEventID, trackingdomain.EventTypeClick)
			if err != nil {
				log.Error("failed to check existing click event", "error", err)
				return err
			}
			if existing != nil {
				log.Info("duplicate click event, skipping", "tracking_event_id", existing.ID)
				trackingEventResult = &RecordClickResult{TrackingEventID: existing.ID, DestinationURL: destinationURL}
				return nil
			}
		}

		eventID, err := s.idGen()
		if err != nil {
			log.Error("failed to generate event id", "error", err)
			return err
		}

		event := trackingdomain.TrackingEvent{
			ID:                eventID,
			WorkspaceID:       resolvedWorkspaceID,
			MessageID:         resolvedMessageID,
			TrackingLinkID:    input.TrackingID,
			EventType:         trackingdomain.EventTypeClick,
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

		if err := s.eventWriteRepo.Create(txCtx, event); err != nil {
			if errors.Is(err, trackingdomain.ErrTrackingEventConflict) {
				log.Info("click event conflict (idempotent), skipping")
				return nil
			}
			log.Error("failed to create click tracking event", "error", err)
			return err
		}

		outboxEventID, err := s.idGen()
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
			AggregateType: "tracking_event",
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

		if err := s.outboxWriter.Save(txCtx, ports.OutboxEvent{
			ID:            outboxEventID,
			AggregateType: "tracking_event",
			AggregateID:   eventID,
			EventType:     contracts.EventLinkClickedV1,
			Payload:       payloadBytes,
			WorkspaceID:   resolvedWorkspaceID,
			OccurredAt:    now,
		}); err != nil {
			log.Error("failed to save outbox event", "error", err)
			return err
		}

		trackingEventResult = &RecordClickResult{TrackingEventID: eventID, DestinationURL: destinationURL}
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

type RecordUnsubscribeInput struct {
	Token             string
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

type RecordUnsubscribeResult struct {
	TrackingEventID string
	SuppressionID   string
}

func (s *Service) RecordUnsubscribe(ctx context.Context, input RecordUnsubscribeInput) (*RecordUnsubscribeResult, error) {
	log := s.log.With("usecase", "record_unsubscribe")

	now := time.Now().UTC()
	if input.ReceivedAt.IsZero() {
		input.ReceivedAt = now
	}
	if input.OccurredAt.IsZero() {
		input.OccurredAt = now
	}

	source := input.Source
	if source == "" {
		source = trackingdomain.SourceHTTP
	}

	var resolvedWorkspaceID, resolvedMessageID, resolvedCampaignID, resolvedRecipientEmailNormalized string

	if input.Token != "" {
		if s.tokenSigner == nil {
			log.Warn("token signer not configured, cannot verify unsubscribe token")
			return nil, &platformerrors.NonRetryableError{Err: trackingdomain.ErrUnsubscribeTokenInvalid}
		}
		payload, err := s.tokenSigner.Verify(input.Token)
		if err != nil {
			if errors.Is(err, unsubscribetoken.ErrTokenExpired) {
				log.Info("unsubscribe token expired")
				return &RecordUnsubscribeResult{}, nil
			}
			log.Info("unsubscribe token invalid")
			return &RecordUnsubscribeResult{}, nil
		}
		resolvedWorkspaceID = payload.WorkspaceID
		resolvedMessageID = payload.MessageID
		resolvedRecipientEmailNormalized = payload.RecipientEmailNormalized
	}

	if resolvedMessageID == "" && input.WorkspaceID != "" && input.MessageID != "" {
		msgID, wsID, campaignID, _, _, emailNorm, err := s.messageResolver.FindByID(ctx, input.WorkspaceID, input.MessageID)
		if err != nil {
			log.Info("message not found for unsubscribe event", "workspace_id", input.WorkspaceID, "message_id", input.MessageID)
			return nil, nil
		}
		resolvedWorkspaceID = wsID
		resolvedMessageID = msgID
		resolvedCampaignID = campaignID
		if resolvedRecipientEmailNormalized == "" {
			resolvedRecipientEmailNormalized = emailNorm
		}
	}

	if resolvedMessageID == "" && input.Provider != "" && input.ProviderMessageID != "" {
		msgID, wsID, campaignID, err := s.messageResolver.FindByProviderMessageID(ctx, input.Provider, input.ProviderMessageID)
		if err != nil {
			log.Info("message not found by provider message id for unsubscribe event", "provider", input.Provider, "provider_message_id", input.ProviderMessageID)
			return nil, nil
		}
		resolvedWorkspaceID = wsID
		resolvedMessageID = msgID
		resolvedCampaignID = campaignID
	}

	if resolvedMessageID == "" {
		log.Info("no message resolved for unsubscribe event, skipping")
		return nil, nil
	}

	if resolvedRecipientEmailNormalized == "" {
		log.Info("cannot unsubscribe: no recipient email resolved")
		return nil, nil
	}

	var result *RecordUnsubscribeResult

	if err := s.txManager.WithinTx(ctx, func(txCtx context.Context) error {
		var trackingEventID string
		if source == trackingdomain.SourceHTTP || input.SourceEventID != "" {
			if input.SourceEventID != "" {
				existing, err := s.eventReadRepo.FindBySourceEvent(txCtx, source, input.SourceEventID, trackingdomain.EventTypeUnsubscribe)
				if err != nil {
					log.Error("failed to check existing unsubscribe event", "error", err)
					return err
				}
				if existing != nil {
					trackingEventID = existing.ID
				}
			}

			if trackingEventID == "" {
				eID, err := s.idGen()
				if err != nil {
					log.Error("failed to generate event id", "error", err)
					return err
				}

				event := trackingdomain.TrackingEvent{
					ID:            eID,
					WorkspaceID:   resolvedWorkspaceID,
					MessageID:     resolvedMessageID,
					EventType:     trackingdomain.EventTypeUnsubscribe,
					Source:        source,
					SourceEventID: input.SourceEventID,
					OccurredAt:    input.OccurredAt,
					ReceivedAt:    input.ReceivedAt,
					Metadata:      map[string]any{},
					CreatedAt:     now,
				}

				if err := s.eventWriteRepo.Create(txCtx, event); err != nil {
					if !errors.Is(err, trackingdomain.ErrTrackingEventConflict) {
						log.Error("failed to create unsubscribe tracking event", "error", err)
						return err
					}
				} else {
					trackingEventID = eID
				}
			}
		}

		var suppressionEntryID string
		if s.recipientSuppressor != nil {
			supResult, supErr := s.recipientSuppressor.SuppressFromSignal(txCtx, SuppressFromSignalInput{
				WorkspaceID:     resolvedWorkspaceID,
				EmailNormalized: resolvedRecipientEmailNormalized,
				Scope:           "workspace",
				Reason:          suppressioncontracts.ReasonUnsubscribe,
				Source:          source,
				SourceEventID:   input.SourceEventID,
				Note:            "unsubscribed via tracking endpoint or provider event",
				Now:             now,
			})
			if supErr != nil {
				log.Warn("suppression creation failed for unsubscribe", "error", supErr)
				return supErr
			}
			suppressionEntryID = supResult.EntryID
		}

		if trackingEventID != "" {
			outboxEventID, err := s.idGen()
			if err != nil {
				log.Error("failed to generate outbox event id", "error", err)
				return err
			}

			payload := contracts.RecipientUnsubscribedPayload{
				TrackingEventID: trackingEventID,
				SuppressionID:   suppressionEntryID,
				WorkspaceID:     resolvedWorkspaceID,
				MessageID:       resolvedMessageID,
				CampaignID:      resolvedCampaignID,
				Source:          source,
				SourceEventID:   input.SourceEventID,
				OccurredAt:      input.OccurredAt.Format(time.RFC3339),
				ReceivedAt:      input.ReceivedAt.Format(time.RFC3339),
			}

			envelope, err := events.NewEnvelope(events.NewEnvelopeOptions{
				EventID:       outboxEventID,
				EventType:     contracts.EventRecipientUnsubscribedV1,
				EventVersion:  1,
				AggregateType: "tracking_event",
				AggregateID:   trackingEventID,
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

			if err := s.outboxWriter.Save(txCtx, ports.OutboxEvent{
				ID:            outboxEventID,
				AggregateType: "tracking_event",
				AggregateID:   trackingEventID,
				EventType:     contracts.EventRecipientUnsubscribedV1,
				Payload:       payloadBytes,
				WorkspaceID:   resolvedWorkspaceID,
				OccurredAt:    now,
			}); err != nil {
				log.Error("failed to save outbox event", "error", err)
				return err
			}
		}

		result = &RecordUnsubscribeResult{
			TrackingEventID: trackingEventID,
			SuppressionID:   suppressionEntryID,
		}
		return nil
	}); err != nil {
		log.Error("record unsubscribe transaction failed", "error", err)
		return nil, err
	}

	log.Info("unsubscribe recorded",
		"tracking_event_id", result.TrackingEventID,
		"suppression_id", result.SuppressionID,
		"workspace_id", resolvedWorkspaceID,
		"message_id", resolvedMessageID,
	)
	return result, nil
}

type HandleProviderEventInput struct {
	EventID           string
	NormalizedEventID string
	RawEventID        string
	WorkspaceID       string
	MessageID         string
	Provider          string
	ProviderEventID   string
	ProviderMessageID string
	EventType         string
	OccurredAt        time.Time
	ReceivedAt        time.Time
}

type HandleProviderEventResult struct {
	Handled bool
	Ignored bool
}

func (s *Service) HandleProviderEvent(ctx context.Context, input HandleProviderEventInput) (*HandleProviderEventResult, error) {
	log := s.log.With(
		"usecase", "handle_provider_event",
		"event_id", input.EventID,
		"normalized_event_id", input.NormalizedEventID,
		"provider", input.Provider,
		"event_type", input.EventType,
	)

	if input.EventID == "" || input.EventType == "" {
		return nil, &platformerrors.NonRetryableError{Err: trackingdomain.ErrTrackingEventInvalid}
	}

	switch input.EventType {
	case "opened":
		_, err := s.RecordOpen(ctx, RecordOpenInput{
			WorkspaceID:       input.WorkspaceID,
			MessageID:         input.MessageID,
			Provider:          input.Provider,
			ProviderMessageID: input.ProviderMessageID,
			ProviderEventID:   input.ProviderEventID,
			NormalizedEventID: input.NormalizedEventID,
			Source:            trackingdomain.SourceProviderEvent,
			SourceEventID:     input.NormalizedEventID,
			OccurredAt:        input.OccurredAt,
			ReceivedAt:        input.ReceivedAt,
		})
		if err != nil {
			return nil, err
		}
		return &HandleProviderEventResult{Handled: true}, nil

	case "clicked":
		_, err := s.RecordClick(ctx, RecordClickInput{
			WorkspaceID:       input.WorkspaceID,
			MessageID:         input.MessageID,
			Provider:          input.Provider,
			ProviderMessageID: input.ProviderMessageID,
			ProviderEventID:   input.ProviderEventID,
			NormalizedEventID: input.NormalizedEventID,
			Source:            trackingdomain.SourceProviderEvent,
			SourceEventID:     input.NormalizedEventID,
			OccurredAt:        input.OccurredAt,
			ReceivedAt:        input.ReceivedAt,
		})
		if err != nil {
			return nil, err
		}
		return &HandleProviderEventResult{Handled: true}, nil

	case "unsubscribed":
		_, err := s.RecordUnsubscribe(ctx, RecordUnsubscribeInput{
			WorkspaceID:       input.WorkspaceID,
			MessageID:         input.MessageID,
			Provider:          input.Provider,
			ProviderMessageID: input.ProviderMessageID,
			ProviderEventID:   input.ProviderEventID,
			NormalizedEventID: input.NormalizedEventID,
			Source:            trackingdomain.SourceProviderEvent,
			SourceEventID:     input.NormalizedEventID,
			OccurredAt:        input.OccurredAt,
			ReceivedAt:        input.ReceivedAt,
		})
		if err != nil {
			return nil, err
		}
		return &HandleProviderEventResult{Handled: true}, nil

	default:
		log.Info("event type not recognized for tracking, ignoring")
		return &HandleProviderEventResult{Ignored: true}, nil
	}
}
