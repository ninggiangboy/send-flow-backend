package campaign

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/contracts"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/ports"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/batching"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/events"
)

type QueueCampaignMessagesInput struct {
	WorkspaceID       string
	CampaignID        string
	TemplateID        string
	TemplateVersionID string
	SenderDomainID    string
	MessageType       string
	ScheduledAt       time.Time
	Now               time.Time
}

type QueueCampaignMessagesResult struct {
	QueuedCount    int
	CandidateCount int
}

type QueueCampaignMessagesHandler struct {
	campaignReader ports.CampaignCandidateReader
	messagesWrite  ports.MessageWriteRepository
	outboxWriter   ports.OutboxWriter
	txManager      ports.UnitOfWork
	idGen          func() (string, error)
	log            *slog.Logger
}

func NewQueueCampaignMessagesHandler(
	campaignReader ports.CampaignCandidateReader,
	messagesWrite ports.MessageWriteRepository,
	outboxWriter ports.OutboxWriter,
	txManager ports.UnitOfWork,
	idGen func() (string, error),
	logger *slog.Logger,
) *QueueCampaignMessagesHandler {
	return &QueueCampaignMessagesHandler{
		campaignReader: campaignReader,
		messagesWrite:  messagesWrite,
		outboxWriter:   outboxWriter,
		txManager:      txManager,
		idGen:          idGen,
		log:            logger.With("usecase", "queue_campaign_messages"),
	}
}

func (h *QueueCampaignMessagesHandler) Execute(ctx context.Context, input QueueCampaignMessagesInput) (*QueueCampaignMessagesResult, error) {
	log := h.log.With("workspace_id", input.WorkspaceID, "campaign_id", input.CampaignID)

	if input.WorkspaceID == "" || input.CampaignID == "" || input.TemplateID == "" || input.TemplateVersionID == "" || input.SenderDomainID == "" || input.MessageType == "" {
		return nil, domain.ErrPayloadInvalid
	}
	if input.Now.IsZero() {
		return nil, domain.ErrPayloadInvalid
	}

	count, err := h.campaignReader.CountCandidates(ctx, input.WorkspaceID, input.CampaignID)
	if err != nil {
		log.Error("failed to count candidates", "error", err)
		return nil, err
	}
	if count == 0 {
		return nil, domain.ErrCampaignCandidatesNotFound
	}

	var totalQueued int
	var cursor string
	var pending []ports.CampaignCandidate
	exhausted := false
	pageSize := 500

	pipeline, err := batching.NewPipeline[ports.CampaignCandidate, domain.Message](batching.Config[ports.CampaignCandidate, domain.Message]{
		Options: batching.Options{
			BufferedItemsSize:    pageSize,
			WriteBatchSize:       pageSize,
			ProcessorConcurrency: 1,
			MaxInflight:          pageSize,
		},
		Reader: batching.ItemReaderFunc[ports.CampaignCandidate](func(ctx context.Context, _ int, _ int) (ports.CampaignCandidate, bool, error) {
			for len(pending) == 0 {
				if exhausted {
					return ports.CampaignCandidate{}, false, nil
				}
				candidates, nextCursor, err := h.campaignReader.ListCandidates(ctx, input.WorkspaceID, input.CampaignID, pageSize, cursor)
				if err != nil {
					log.Error("failed to list candidates", "cursor", cursor, "error", err)
					return ports.CampaignCandidate{}, false, err
				}
				if len(candidates) == 0 {
					return ports.CampaignCandidate{}, false, nil
				}
				pending = candidates
				if nextCursor == "" {
					exhausted = true
				}
				cursor = nextCursor
			}

			candidate := pending[0]
			pending = pending[1:]
			return candidate, true, nil
		}),
		Processor: batching.ItemProcessorFunc[ports.CampaignCandidate, domain.Message](func(ctx context.Context, c ports.CampaignCandidate) (domain.Message, bool, error) {
			var snapshot domain.RecipientSnapshot
			if len(c.RecipientSnapshot) > 0 {
				if err := json.Unmarshal(c.RecipientSnapshot, &snapshot); err != nil {
					log.Warn("failed to unmarshal recipient snapshot", "candidate_id", c.ID, "error", err)
				}
			}

			msgID, err := h.idGen()
			if err != nil {
				log.Error("failed to generate message ID", "error", err)
				return domain.Message{}, false, err
			}

			return domain.Message{
				ID:                       msgID,
				WorkspaceID:              input.WorkspaceID,
				CampaignID:               input.CampaignID,
				CampaignCandidateID:      c.ID,
				ContactID:                c.ContactID,
				RecipientEmailNormalized: c.EmailNormalized,
				RecipientSnapshot:        snapshot,
				TemplateID:               input.TemplateID,
				TemplateVersionID:        input.TemplateVersionID,
				SenderDomainID:           input.SenderDomainID,
				MessageType:              input.MessageType,
				SourceType:               domain.MessageSourceCampaign,
				Status:                   domain.MessageStatusQueued,
				ScheduledAt:              &input.ScheduledAt,
				QueuedAt:                 &input.Now,
				CreatedAt:                input.Now,
				UpdatedAt:                input.Now,
			}, true, nil
		}),
		Writer: batching.ItemWriterFunc[domain.Message](func(ctx context.Context, messages []domain.Message) error {
			var pageQueued int
			if err := h.txManager.WithinTx(ctx, func(txCtx context.Context) error {
				insertedIDs, err := h.messagesWrite.CreateMany(txCtx, messages)
				if err != nil {
					return err
				}
				pageQueued = len(insertedIDs)

				insertedSet := make(map[string]struct{}, len(insertedIDs))
				for _, id := range insertedIDs {
					insertedSet[id] = struct{}{}
				}

				for _, msg := range messages {
					if _, ok := insertedSet[msg.ID]; !ok {
						continue
					}

					eventID, err := h.idGen()
					if err != nil {
						return err
					}

					var scheduledAt string
					if msg.ScheduledAt != nil {
						scheduledAt = msg.ScheduledAt.Format(time.RFC3339)
					}

					payload := contracts.MessageQueuedPayload{
						MessageID:           msg.ID,
						WorkspaceID:         msg.WorkspaceID,
						CampaignID:          msg.CampaignID,
						CampaignCandidateID: msg.CampaignCandidateID,
						TemplateID:          msg.TemplateID,
						TemplateVersionID:   msg.TemplateVersionID,
						SenderDomainID:      msg.SenderDomainID,
						MessageType:         msg.MessageType,
						SourceType:          msg.SourceType,
						ScheduledAt:         scheduledAt,
					}
					envelope, err := events.NewEnvelope(events.NewEnvelopeOptions{
						EventID:       eventID,
						EventType:     contracts.EventDeliveryMessageQueuedV1,
						EventVersion:  1,
						AggregateType: "message",
						AggregateID:   msg.ID,
						WorkspaceID:   msg.WorkspaceID,
						OccurredAt:    input.Now,
					}, payload)
					if err != nil {
						return err
					}
					payloadBytes, err := events.Marshal(envelope)
					if err != nil {
						return err
					}
					if err := h.outboxWriter.Save(txCtx, ports.OutboxEvent{
						ID:            envelope.EventID,
						AggregateType: "message",
						AggregateID:   msg.ID,
						EventType:     contracts.EventDeliveryMessageQueuedV1,
						Payload:       payloadBytes,
						WorkspaceID:   msg.WorkspaceID,
						OccurredAt:    input.Now,
					}); err != nil {
						return err
					}
				}
				return nil
			}); err != nil {
				log.Error("failed to queue messages page", "error", err)
				return err
			}
			totalQueued += pageQueued
			return nil
		}),
	})
	if err != nil {
		return nil, err
	}
	if err := pipeline.Run(ctx); err != nil {
		return nil, err
	}

	log.Info("campaign messages queued",
		"queued_count", totalQueued,
		"candidate_count", count,
	)

	return &QueueCampaignMessagesResult{
		QueuedCount:    totalQueued,
		CandidateCount: int(count),
	}, nil
}
