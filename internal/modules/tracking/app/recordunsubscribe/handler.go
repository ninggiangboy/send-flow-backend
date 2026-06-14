package recordunsubscribe

import (
	"context"
	"errors"
	"log/slog"
	"time"

	suppressioncontracts "github.com/ninggiangboy/send-flow/backend/internal/modules/suppression/contracts"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/tracking/app/unsubscribetoken"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/tracking/contracts"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/tracking/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/tracking/ports"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/events"
	platformerrors "github.com/ninggiangboy/send-flow/backend/internal/platform/retryable"
)

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

type RecipientSuppressor interface {
	SuppressFromSignal(ctx context.Context, input SuppressFromSignalInput) (*SuppressFromSignalResult, error)
}

type Options struct {
	EventReadRepo       ports.TrackingEventReadRepository
	EventWriteRepo      ports.TrackingEventWriteRepository
	MessageResolver     ports.DeliveryMessageResolver
	RecipientSuppressor RecipientSuppressor
	OutboxWriter        ports.OutboxWriter
	TxManager           ports.TransactionManager
	IDGen               func() (string, error)
	TokenSigner         *unsubscribetoken.Signer
	Logger              *slog.Logger
}

type Handler struct {
	eventReadRepo       ports.TrackingEventReadRepository
	eventWriteRepo      ports.TrackingEventWriteRepository
	messageResolver     ports.DeliveryMessageResolver
	recipientSuppressor RecipientSuppressor
	outboxWriter        ports.OutboxWriter
	txManager           ports.TransactionManager
	idGen               func() (string, error)
	tokenSigner         *unsubscribetoken.Signer
	log                 *slog.Logger
}

func New(opts Options) *Handler {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	return &Handler{
		eventReadRepo:       opts.EventReadRepo,
		eventWriteRepo:      opts.EventWriteRepo,
		messageResolver:     opts.MessageResolver,
		recipientSuppressor: opts.RecipientSuppressor,
		outboxWriter:        opts.OutboxWriter,
		txManager:           opts.TxManager,
		idGen:               opts.IDGen,
		tokenSigner:         opts.TokenSigner,
		log:                 opts.Logger.With("usecase", "record_unsubscribe"),
	}
}

type Input struct {
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

type Result struct {
	TrackingEventID string
	SuppressionID   string
}

func (h *Handler) Execute(ctx context.Context, input Input) (*Result, error) {
	log := h.log.With("source", input.Source)

	now := time.Now().UTC()
	if input.ReceivedAt.IsZero() {
		input.ReceivedAt = now
	}
	if input.OccurredAt.IsZero() {
		input.OccurredAt = now
	}

	source := input.Source
	if source == "" {
		source = domain.SourceHTTP
	}

	var resolvedWorkspaceID, resolvedMessageID, resolvedCampaignID, resolvedRecipientEmailNormalized string

	if input.Token != "" {
		if h.tokenSigner == nil {
			log.Warn("token signer not configured, cannot verify unsubscribe token")
			return nil, &platformerrors.NonRetryableError{Err: domain.ErrUnsubscribeTokenInvalid}
		}
		payload, err := h.tokenSigner.Verify(input.Token)
		if err != nil {
			if errors.Is(err, unsubscribetoken.ErrTokenExpired) {
				log.Info("unsubscribe token expired")
				return &Result{}, nil
			}
			log.Info("unsubscribe token invalid")
			return &Result{}, nil
		}
		resolvedWorkspaceID = payload.WorkspaceID
		resolvedMessageID = payload.MessageID
		resolvedRecipientEmailNormalized = payload.RecipientEmailNormalized
	}

	if resolvedMessageID == "" && input.WorkspaceID != "" && input.MessageID != "" {
		msgID, wsID, campaignID, _, _, emailNorm, err := h.messageResolver.FindByID(ctx, input.WorkspaceID, input.MessageID)
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
		msgID, wsID, campaignID, err := h.messageResolver.FindByProviderMessageID(ctx, input.Provider, input.ProviderMessageID)
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

	var result *Result

	if err := h.txManager.WithinTx(ctx, func(txCtx context.Context) error {
		var trackingEventID string
		if source == domain.SourceHTTP || input.SourceEventID != "" {
			if input.SourceEventID != "" {
				existing, err := h.eventReadRepo.FindBySourceEvent(txCtx, source, input.SourceEventID, domain.EventTypeUnsubscribe)
				if err != nil {
					log.Error("failed to check existing unsubscribe event", "error", err)
					return err
				}
				if existing != nil {
					trackingEventID = existing.ID
				}
			}

			if trackingEventID == "" {
				eID, err := h.idGen()
				if err != nil {
					log.Error("failed to generate event id", "error", err)
					return err
				}

				event := domain.TrackingEvent{
					ID:            eID,
					WorkspaceID:   resolvedWorkspaceID,
					MessageID:     resolvedMessageID,
					EventType:     domain.EventTypeUnsubscribe,
					Source:        source,
					SourceEventID: input.SourceEventID,
					OccurredAt:    input.OccurredAt,
					ReceivedAt:    input.ReceivedAt,
					Metadata:      map[string]any{},
					CreatedAt:     now,
				}

				if err := h.eventWriteRepo.Create(txCtx, event); err != nil {
					if !errors.Is(err, domain.ErrTrackingEventConflict) {
						log.Error("failed to create unsubscribe tracking event", "error", err)
						return err
					}
				} else {
					trackingEventID = eID
				}
			}
		}

		var suppressionEntryID string
		if h.recipientSuppressor != nil {
			supResult, supErr := h.recipientSuppressor.SuppressFromSignal(txCtx, SuppressFromSignalInput{
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
			outboxEventID, err := h.idGen()
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

			if err := h.outboxWriter.Save(txCtx, ports.OutboxEvent{
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

		result = &Result{
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
