package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/ingestion/contracts"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/ingestion/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/ingestion/ports"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/events"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/id"
)

const maxRawBodySize = 256 * 1024

type Options struct {
	RawEventsRead         ports.RawEventReadRepository
	RawEventsWrite        ports.RawEventWriteRepository
	NormalizedEventsRead  ports.NormalizedEventReadRepository
	NormalizedEventsWrite ports.NormalizedEventWriteRepository
	ProviderRegistry      ports.ProviderRegistry
	MessageResolver       ports.DeliveryMessageResolver
	OutboxWriter          ports.OutboxWriter
	TxManager             ports.TransactionManager
	IDGen                 func() (string, error)
	Logger                *slog.Logger
}

type Service struct {
	rawEventsRead         ports.RawEventReadRepository
	rawEventsWrite        ports.RawEventWriteRepository
	normalizedEventsRead  ports.NormalizedEventReadRepository
	normalizedEventsWrite ports.NormalizedEventWriteRepository
	providerRegistry      ports.ProviderRegistry
	messageResolver       ports.DeliveryMessageResolver
	outboxWriter          ports.OutboxWriter
	txManager             ports.TransactionManager
	idGen                 func() (string, error)
	log                   *slog.Logger
}

func NewService(opts Options) *Service {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.IDGen == nil {
		opts.IDGen = id.NewUUIDGenerator().New
	}
	return &Service{
		rawEventsRead:         opts.RawEventsRead,
		rawEventsWrite:        opts.RawEventsWrite,
		normalizedEventsRead:  opts.NormalizedEventsRead,
		normalizedEventsWrite: opts.NormalizedEventsWrite,
		providerRegistry:      opts.ProviderRegistry,
		messageResolver:       opts.MessageResolver,
		outboxWriter:          opts.OutboxWriter,
		txManager:             opts.TxManager,
		idGen:                 opts.IDGen,
		log:                   opts.Logger.With("module", "ingestion"),
	}
}

type IngestProviderWebhookInput struct {
	Provider   string
	Headers    map[string][]string
	RawBody    []byte
	ReceivedAt time.Time
}

type IngestProviderWebhookResult struct {
	Accepted          bool
	RawEventID        string
	NormalizedEventID string
}

func (s *Service) IngestProviderWebhook(ctx context.Context, input IngestProviderWebhookInput) (*IngestProviderWebhookResult, error) {
	provider := strings.TrimSpace(strings.ToLower(input.Provider))
	log := s.log.With("usecase", "ingest_provider_webhook", "provider", provider)

	if provider == "" {
		return nil, domain.ErrProviderNotSupported
	}

	verifier, ok := s.providerRegistry.Verifier(provider)
	if !ok {
		log.Warn("unsupported provider")
		return nil, domain.ErrProviderNotSupported
	}
	normalizer, ok := s.providerRegistry.Normalizer(provider)
	if !ok {
		log.Warn("provider missing normalizer")
		return nil, domain.ErrProviderNotSupported
	}

	if len(input.RawBody) > maxRawBodySize {
		log.Warn("payload exceeds size limit")
		return nil, domain.ErrPayloadInvalid
	}

	if !json.Valid(bytes.TrimSpace(input.RawBody)) {
		log.Warn("invalid JSON payload")
		return nil, domain.ErrPayloadInvalid
	}

	sigErr := verifier.Verify(ctx, ports.VerifyInput{
		Provider: provider,
		Headers:  input.Headers,
		RawBody:  input.RawBody,
	})
	if sigErr != nil {
		if errors.Is(sigErr, domain.ErrInvalidSignature) {
			log.Warn("invalid webhook signature")
			return nil, domain.ErrInvalidSignature
		}
		log.Error("signature verification failed", "error", sigErr)
		return nil, domain.ErrTemporarilyUnavailable
	}

	normalized, normErr := normalizer.Normalize(ctx, ports.NormalizeInput{
		Provider: provider,
		RawBody:  input.RawBody,
	})

	var rawEventID string
	var normalizedEventID string
	var workspaceID, messageID string

	if err := s.txManager.WithinTx(ctx, func(txCtx context.Context) error {
		now := input.ReceivedAt
		if now.IsZero() {
			now = time.Now().UTC()
		}

		var idErr error
		rawEventID, idErr = s.idGen()
		if idErr != nil {
			return idErr
		}
		if normErr == nil && normalized.ProviderMessageID != "" {
			ref, resolveErr := s.messageResolver.FindByProviderMessageID(txCtx, provider, normalized.ProviderMessageID)
			if resolveErr == nil && ref != nil {
				workspaceID = ref.WorkspaceID
				messageID = ref.MessageID
			}
		}

		headersSafe := make(map[string][]string, len(input.Headers))
		for k, v := range input.Headers {
			kl := strings.ToLower(k)
			switch kl {
			case "content-type", "user-agent", "x-sendflow-fake-signature",
				"x-amz-sns-message-type", "x-amz-sns-message-id",
				"x-amz-sns-topic-arn", "x-amz-sns-subscription-arn",
				"x-forwarded-for", "x-forwarded-proto", "x-real-ip":
				headersSafe[k] = v
			}
		}
		headersJSON, _ := json.Marshal(headersSafe)

		rawEvent := domain.ProviderWebhookEvent{
			ID:                rawEventID,
			Provider:          provider,
			ProviderEventID:   "",
			ProviderMessageID: "",
			WorkspaceID:       workspaceID,
			MessageID:         messageID,
			EventType:         "",
			PayloadJSON:       input.RawBody,
			HeadersJSON:       headersJSON,

			SignatureValid: true,
			ReceivedAt:     now,
			CreatedAt:      now,
		}
		if normErr == nil && normalized != nil {
			rawEvent.ProviderEventID = normalized.ProviderEventID
			rawEvent.ProviderMessageID = normalized.ProviderMessageID
			rawEvent.EventType = normalized.EventType
		}

		if err := s.rawEventsWrite.Create(txCtx, rawEvent); err != nil {
			if errors.Is(err, domain.ErrDuplicateEventConflict) {
				existing, findErr := s.rawEventsRead.FindByProviderEventID(txCtx, provider, rawEvent.ProviderEventID)
				if findErr == nil && existing != nil {
					rawEventID = existing.ID
					_ = existing
					return nil
				}
			}
			log.Error("failed to persist raw event", "error", err)
			return err
		}

		receivedPayload := contracts.ProviderWebhookReceivedPayload{
			RawEventID:        rawEventID,
			Provider:          provider,
			ProviderEventID:   rawEvent.ProviderEventID,
			ProviderMessageID: rawEvent.ProviderMessageID,
			WorkspaceID:       workspaceID,
			MessageID:         messageID,
			ReceivedAt:        now.Format(time.RFC3339),
		}
		envelopeEventID, err := s.idGen()
		if err != nil {
			return err
		}
		envelope, err := events.NewEnvelope(events.NewEnvelopeOptions{
			EventID:       envelopeEventID,
			EventType:     contracts.EventProviderWebhookReceivedV1,
			EventVersion:  1,
			AggregateType: "provider_webhook_event",
			AggregateID:   rawEventID,
			WorkspaceID:   workspaceID,
			OccurredAt:    now,
		}, receivedPayload)
		if err != nil {
			log.Error("failed to create received envelope", "error", err)
			return err
		}
		payloadBytes, err := events.Marshal(envelope)
		if err != nil {
			log.Error("failed to marshal received event", "error", err)
			return err
		}
		if err := s.outboxWriter.Save(txCtx, ports.OutboxEvent{
			ID:            envelope.EventID,
			AggregateType: "provider_webhook_event",
			AggregateID:   rawEventID,
			EventType:     contracts.EventProviderWebhookReceivedV1,
			Payload:       payloadBytes,
			WorkspaceID:   workspaceID,
			OccurredAt:    now,
		}); err != nil {
			log.Error("failed to save received outbox event", "error", err)
			return err
		}

		if normErr == nil && normalized != nil {
			var nidErr error
			normalizedEventID, nidErr = s.idGen()
			if nidErr != nil {
				return nidErr
			}

			normEvent := domain.NormalizedProviderEvent{
				ID:                normalizedEventID,
				RawEventID:        rawEventID,
				WorkspaceID:       workspaceID,
				MessageID:         messageID,
				Provider:          provider,
				ProviderEventID:   normalized.ProviderEventID,
				ProviderMessageID: normalized.ProviderMessageID,
				EventType:         normalized.EventType,
				OccurredAt:        normalized.OccurredAt,
				ReceivedAt:        now,
				PayloadJSON:       normalized.PayloadJSON,
				CreatedAt:         now,
			}

			if err := s.normalizedEventsWrite.Create(txCtx, normEvent); err != nil {
				if errors.Is(err, domain.ErrDuplicateEventConflict) {
					existing, findErr := s.normalizedEventsRead.FindByProviderEventID(txCtx, provider, normalized.ProviderEventID, normalized.EventType)
					if findErr == nil && existing != nil {
						normalizedEventID = existing.ID
						return nil
					}
				}
				log.Error("failed to persist normalized event", "error", err)
				return err
			}

			normalizedPayload := contracts.ProviderEventNormalizedPayload{
				NormalizedEventID: normalizedEventID,
				RawEventID:        rawEventID,
				Provider:          provider,
				ProviderEventID:   normalized.ProviderEventID,
				ProviderMessageID: normalized.ProviderMessageID,
				WorkspaceID:       workspaceID,
				MessageID:         messageID,
				EventType:         normalized.EventType,
				OccurredAt:        normalized.OccurredAt.Format(time.RFC3339),
				ReceivedAt:        now.Format(time.RFC3339),
			}
			normEnvelopeEventID, err := s.idGen()
			if err != nil {
				return err
			}
			normEnvelope, err := events.NewEnvelope(events.NewEnvelopeOptions{
				EventID:       normEnvelopeEventID,
				EventType:     contracts.EventProviderEventNormalizedV1,
				EventVersion:  1,
				AggregateType: "normalized_provider_event",
				AggregateID:   normalizedEventID,
				WorkspaceID:   workspaceID,
				OccurredAt:    now,
			}, normalizedPayload)
			if err != nil {
				log.Error("failed to create normalized envelope", "error", err)
				return err
			}
			normPayloadBytes, err := events.Marshal(normEnvelope)
			if err != nil {
				log.Error("failed to marshal normalized event", "error", err)
				return err
			}
			if err := s.outboxWriter.Save(txCtx, ports.OutboxEvent{
				ID:            normEnvelope.EventID,
				AggregateType: "normalized_provider_event",
				AggregateID:   normalizedEventID,
				EventType:     contracts.EventProviderEventNormalizedV1,
				Payload:       normPayloadBytes,
				WorkspaceID:   workspaceID,
				OccurredAt:    now,
			}); err != nil {
				log.Error("failed to save normalized outbox event", "error", err)
				return err
			}
		}

		return nil
	}); err != nil {
		if errors.Is(err, domain.ErrPayloadInvalid) || errors.Is(err, domain.ErrProviderNotSupported) ||
			errors.Is(err, domain.ErrInvalidSignature) || errors.Is(err, domain.ErrDuplicateEventConflict) {
			return nil, err
		}
		log.Error("ingestion transaction failed", "error", err)
		return nil, domain.ErrTemporarilyUnavailable
	}

	log.Info("provider webhook accepted",
		"raw_event_id", rawEventID,
		"normalized_event_id", normalizedEventID,
		"message_id", messageID,
		"workspace_id", workspaceID,
	)
	_ = log

	return &IngestProviderWebhookResult{
		Accepted:          true,
		RawEventID:        rawEventID,
		NormalizedEventID: normalizedEventID,
	}, nil
}
