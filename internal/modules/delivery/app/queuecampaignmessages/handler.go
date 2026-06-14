package queuecampaignmessages

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/contracts"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/ports"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/events"
)

type Input struct {
	WorkspaceID       string
	CampaignID        string
	TemplateID        string
	TemplateVersionID string
	SenderDomainID    string
	MessageType       string
	ScheduledAt       time.Time
	Now               time.Time
}

type Result struct {
	QueuedCount    int
	CandidateCount int
}

type Handler struct {
	campaignReader ports.CampaignCandidateReader
	messagesWrite  ports.MessageWriteRepository
	outboxWriter   ports.OutboxWriter
	txManager      ports.UnitOfWork
	idGen          func() (string, error)
	log            *slog.Logger
}

func New(
	campaignReader ports.CampaignCandidateReader,
	messagesWrite ports.MessageWriteRepository,
	outboxWriter ports.OutboxWriter,
	txManager ports.UnitOfWork,
	idGen func() (string, error),
	logger *slog.Logger,
) *Handler {
	return &Handler{
		campaignReader: campaignReader,
		messagesWrite:  messagesWrite,
		outboxWriter:   outboxWriter,
		txManager:      txManager,
		idGen:          idGen,
		log:            logger.With("usecase", "queue_campaign_messages"),
	}
}

func (h *Handler) Execute(ctx context.Context, input Input) (*Result, error) {
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
	pageSize := 500

	for {
		candidates, nextCursor, err := h.campaignReader.ListCandidates(ctx, input.WorkspaceID, input.CampaignID, pageSize, cursor)
		if err != nil {
			log.Error("failed to list candidates", "cursor", cursor, "error", err)
			return nil, err
		}
		if len(candidates) == 0 {
			break
		}

		messages := make([]domain.Message, 0, len(candidates))
		for _, c := range candidates {
			var snapshot domain.RecipientSnapshot
			if len(c.RecipientSnapshot) > 0 {
				if err := json.Unmarshal(c.RecipientSnapshot, &snapshot); err != nil {
					log.Warn("failed to unmarshal recipient snapshot", "candidate_id", c.ID, "error", err)
				}
			}

			msgID, err := h.idGen()
			if err != nil {
				log.Error("failed to generate message ID", "error", err)
				return nil, err
			}

			messages = append(messages, domain.Message{
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
			})
		}

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
			return nil, err
		} else {
			totalQueued += pageQueued
		}

		if nextCursor == "" {
			break
		}
		cursor = nextCursor
	}

	log.Info("campaign messages queued",
		"queued_count", totalQueued,
		"candidate_count", count,
	)

	return &Result{
		QueuedCount:    totalQueued,
		CandidateCount: int(count),
	}, nil
}
