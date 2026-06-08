package app

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/contracts"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/ports"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/events"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/id"
)

type Options struct {
	MessagesRead        ports.MessageReadRepository
	MessagesWrite       ports.MessageWriteRepository
	AttemptsRead        ports.AttemptReadRepository
	AttemptsWrite       ports.AttemptWriteRepository
	RetryStatesRead     ports.RetryStateReadRepository
	RetryStatesWrite    ports.RetryStateWriteRepository
	TxRequestsRead      ports.TransactionalRequestReadRepository
	TxRequestsWrite     ports.TransactionalRequestWriteRepository
	CampaignReader      ports.CampaignCandidateReader
	ContentRenderer     ports.ContentRenderer
	SenderChecker       ports.SenderReadinessChecker
	SuppressionChecker  ports.SuppressionChecker
	RecipientSuppressor RecipientSuppressor
	EmailProvider       ports.EmailProvider
	OutboxWriter        ports.OutboxWriter
	TxManager           ports.TransactionManager
	AccessChecker       ports.WorkspaceAccessChecker
	IDGen               func() (string, error)
	Logger              *slog.Logger
}

type Service struct {
	messagesRead        ports.MessageReadRepository
	messagesWrite       ports.MessageWriteRepository
	attemptsRead        ports.AttemptReadRepository
	attemptsWrite       ports.AttemptWriteRepository
	retryStatesRead     ports.RetryStateReadRepository
	retryStatesWrite    ports.RetryStateWriteRepository
	txRequestsRead      ports.TransactionalRequestReadRepository
	txRequestsWrite     ports.TransactionalRequestWriteRepository
	campaignReader      ports.CampaignCandidateReader
	contentRenderer     ports.ContentRenderer
	senderChecker       ports.SenderReadinessChecker
	suppressionChecker  ports.SuppressionChecker
	recipientSuppressor RecipientSuppressor
	emailProvider       ports.EmailProvider
	outboxWriter        ports.OutboxWriter
	txManager           ports.TransactionManager
	accessChecker       ports.WorkspaceAccessChecker
	idGen               func() (string, error)
	log                 *slog.Logger
}

func NewService(opts Options) *Service {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.IDGen == nil {
		opts.IDGen = id.NewUUIDGenerator().New
	}
	return &Service{
		messagesRead:        opts.MessagesRead,
		messagesWrite:       opts.MessagesWrite,
		attemptsRead:        opts.AttemptsRead,
		attemptsWrite:       opts.AttemptsWrite,
		retryStatesRead:     opts.RetryStatesRead,
		retryStatesWrite:    opts.RetryStatesWrite,
		txRequestsRead:      opts.TxRequestsRead,
		txRequestsWrite:     opts.TxRequestsWrite,
		campaignReader:      opts.CampaignReader,
		contentRenderer:     opts.ContentRenderer,
		senderChecker:       opts.SenderChecker,
		suppressionChecker:  opts.SuppressionChecker,
		recipientSuppressor: opts.RecipientSuppressor,
		emailProvider:       opts.EmailProvider,
		outboxWriter:        opts.OutboxWriter,
		txManager:           opts.TxManager,
		accessChecker:       opts.AccessChecker,
		idGen:               opts.IDGen,
		log:                 opts.Logger.With("module", "delivery"),
	}
}

func mustNewID(gen func() (string, error)) string {
	id, err := gen()
	if err != nil {
		panic(err)
	}
	return id
}

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

func (s *Service) QueueCampaignMessages(ctx context.Context, input QueueCampaignMessagesInput) (*QueueCampaignMessagesResult, error) {
	log := s.log.With("usecase", "queue_campaign_messages", "workspace_id", input.WorkspaceID, "campaign_id", input.CampaignID)

	if input.WorkspaceID == "" || input.CampaignID == "" || input.TemplateID == "" || input.TemplateVersionID == "" || input.SenderDomainID == "" || input.MessageType == "" {
		return nil, domain.ErrPayloadInvalid
	}
	if input.Now.IsZero() {
		return nil, domain.ErrPayloadInvalid
	}

	count, err := s.campaignReader.CountCandidates(ctx, input.WorkspaceID, input.CampaignID)
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
		candidates, nextCursor, err := s.campaignReader.ListCandidates(ctx, input.WorkspaceID, input.CampaignID, pageSize, cursor)
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

			messages = append(messages, domain.Message{
				ID:                       mustNewID(s.idGen),
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
		if err := s.txManager.RunInTransaction(ctx, func(txCtx context.Context) error {
			insertedIDs, err := s.messagesWrite.CreateMany(txCtx, messages)
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

				eventID := mustNewID(s.idGen)

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
				if err := s.outboxWriter.Save(txCtx, ports.OutboxEvent{
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

	return &QueueCampaignMessagesResult{
		QueuedCount:    totalQueued,
		CandidateCount: int(count),
	}, nil
}

type ListMessagesInput struct {
	UserID                   string
	WorkspaceID              string
	CampaignID               string
	TransactionalRequestID   string
	Status                   string
	RecipientEmailNormalized string
	ProviderMessageID        string
	From                     *time.Time
	To                       *time.Time
	Limit                    int
	Cursor                   string
}

type ListMessagesResult struct {
	Messages   []domain.Message
	NextCursor string
}

func (s *Service) ListMessages(ctx context.Context, input ListMessagesInput) (*ListMessagesResult, error) {
	if input.WorkspaceID == "" {
		return nil, domain.ErrPayloadInvalid
	}
	if s.accessChecker != nil {
		if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, "delivery.read"); err != nil {
			return nil, err
		}
	}

	limit := input.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	emailNormalized := strings.TrimSpace(strings.ToLower(input.RecipientEmailNormalized))

	query := ports.MessageListQuery{
		WorkspaceID:              input.WorkspaceID,
		CampaignID:               input.CampaignID,
		TransactionalRequestID:   input.TransactionalRequestID,
		Status:                   input.Status,
		RecipientEmailNormalized: emailNormalized,
		ProviderMessageID:        input.ProviderMessageID,
		From:                     input.From,
		To:                       input.To,
		Limit:                    limit,
		Cursor:                   input.Cursor,
	}

	messages, cursor, err := s.messagesRead.List(ctx, query)
	if err != nil {
		s.log.Error("failed to list messages", "workspace_id", input.WorkspaceID, "error", err)
		return nil, err
	}

	return &ListMessagesResult{Messages: messages, NextCursor: cursor}, nil
}

type GetMessageInput struct {
	UserID      string
	WorkspaceID string
	MessageID   string
}

func (s *Service) GetMessage(ctx context.Context, input GetMessageInput) (*domain.Message, error) {
	if input.WorkspaceID == "" || input.MessageID == "" {
		return nil, domain.ErrPayloadInvalid
	}
	if s.accessChecker != nil {
		if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, "delivery.read"); err != nil {
			return nil, err
		}
	}

	msg, err := s.messagesRead.FindByID(ctx, input.WorkspaceID, input.MessageID)
	if err != nil {
		if errors.Is(err, domain.ErrMessageNotFound) {
			return nil, err
		}
		s.log.Error("failed to find message", "workspace_id", input.WorkspaceID, "message_id", input.MessageID, "error", err)
		return nil, err
	}

	return msg, nil
}
