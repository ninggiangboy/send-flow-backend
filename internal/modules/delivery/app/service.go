package app

import (
	"context"
	"log/slog"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/app/campaign"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/app/message"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/app/process"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/app/send"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/domain"
	deliveryredis "github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/infrastructure/redis"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/ports"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/constants"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/id"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/observability"
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
	TxManager           ports.UnitOfWork
	AccessChecker       ports.WorkspaceAccessChecker
	IDGen               func() (string, error)
	Logger              *slog.Logger
	RedisCache          *deliveryredis.Cache
	EventRepo           ports.MessageEventRepository
	AttachmentRepo      ports.AttachmentRepository
	ObjectStorage       ports.ObjectStorage
	AttachmentMetrics   *observability.AttachmentMetrics
	QuotaEnforcer       ports.QuotaEnforcer
}

type Service struct {
	commands     CommandBus
	queries      QueryBus
	log          *slog.Logger
	eventRepo    ports.MessageEventRepository
	messagesRead ports.MessageReadRepository
	attemptsRead ports.AttemptReadRepository
}

func NewService(opts Options) *Service {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.IDGen == nil {
		opts.IDGen = id.NewUUIDGenerator().New
	}
	if opts.MessagesRead == nil {
		opts.MessagesRead = opts.MessagesWrite
	}

	recipientSuppressor := opts.RecipientSuppressor
	if recipientSuppressor == nil {
		// If not provided, create a no-op suppressor for backward compat
		recipientSuppressor = opts.RecipientSuppressor
	}

	acceptTransactionalH := send.New(
		opts.TxRequestsWrite,
		opts.MessagesWrite,
		opts.SenderChecker,
		opts.ContentRenderer,
		opts.SuppressionChecker,
		opts.AttachmentRepo,
		opts.EventRepo,
		opts.ObjectStorage,
		opts.OutboxWriter,
		opts.TxManager,
		opts.IDGen,
		opts.Logger,
		opts.RedisCache,
		opts.AttachmentMetrics,
		opts.QuotaEnforcer,
	)

	queueCampaignH := campaign.NewQueueCampaignMessagesHandler(
		opts.CampaignReader,
		opts.MessagesWrite,
		opts.OutboxWriter,
		opts.TxManager,
		opts.IDGen,
		opts.Logger,
	)

	handleCampaignH := campaign.NewCampaignScheduledHandler(queueCampaignH, opts.Logger)

	handleProviderH := process.NewProviderEventHandler(
		opts.MessagesWrite,
		opts.TxRequestsWrite,
		recipientSuppressor,
		opts.OutboxWriter,
		opts.TxManager,
		opts.IDGen,
		opts.Logger,
		opts.EventRepo,
	)

	processDueMsgsH := process.NewProcessDueMessagesHandler(
		opts.MessagesWrite,
		opts.AttemptsWrite,
		opts.RetryStatesWrite,
		opts.SenderChecker,
		opts.SuppressionChecker,
		opts.ContentRenderer,
		opts.EmailProvider,
		opts.OutboxWriter,
		opts.TxManager,
		opts.TxRequestsWrite,
		opts.IDGen,
		opts.Logger,
		opts.RedisCache,
		opts.EventRepo,
		opts.AttachmentRepo,
		opts.ObjectStorage,
	)

	querySvc := message.NewQueryService(
		opts.MessagesRead,
		opts.AccessChecker,
		opts.Logger,
	)

	return &Service{
		commands: newCommandBus(opts.Logger,
			acceptTransactionalH,
			queueCampaignH,
			handleCampaignH,
			handleProviderH,
			processDueMsgsH,
		),
		queries: newQueryBus(opts.Logger,
			querySvc,
		),
		log:          opts.Logger.With("module", "delivery"),
		eventRepo:    opts.EventRepo,
		messagesRead: opts.MessagesRead,
		attemptsRead: opts.AttemptsRead,
	}
}

// Facade methods delegating to handlers via buses

func (s *Service) QueueCampaignMessages(ctx context.Context, input QueueCampaignMessagesInput) (*QueueCampaignMessagesResult, error) {
	return s.commands.QueueCampaignMessages(ctx, input)
}

func (s *Service) ListMessages(ctx context.Context, input ListMessagesInput) (*ListMessagesResult, error) {
	return s.queries.ListMessages(ctx, input)
}

func (s *Service) GetMessage(ctx context.Context, input GetMessageInput) (*domain.Message, error) {
	return s.queries.GetMessage(ctx, input)
}

func (s *Service) AcceptTransactionalSend(ctx context.Context, input AcceptTransactionalSendInput) (*AcceptTransactionalSendResult, error) {
	return s.commands.AcceptTransactionalSend(ctx, input)
}

func (s *Service) GetTransactionalMessage(ctx context.Context, input GetTransactionalMessageInput) (*GetTransactionalMessageResult, error) {
	return s.queries.GetTransactionalMessage(ctx, input.WorkspaceID, input.MessageID)
}

func (s *Service) HandleCampaignScheduled(ctx context.Context, input HandleCampaignScheduledInput) error {
	return s.commands.HandleCampaignScheduled(ctx, input)
}

func (s *Service) HandleProviderEvent(ctx context.Context, input HandleProviderEventInput) (*HandleProviderEventResult, error) {
	return s.commands.HandleProviderEvent(ctx, input)
}

func (s *Service) ProcessDueMessagesAllWorkspaces(ctx context.Context, input ProcessDueMessagesAllInput) (int, error) {
	return s.commands.ProcessDueMessagesAllWorkspaces(ctx, input)
}

func (s *Service) ListMessageEvents(ctx context.Context, input ListMessageEventsInput) (*ListMessageEventsResult, error) {
	limit := input.Limit
	if limit <= 0 {
		limit = constants.DefaultPageSize
	}

	events, cursor, err := s.eventRepo.ListByMessage(ctx, input.WorkspaceID, input.MessageID, limit, input.Cursor)
	if err != nil {
		return nil, err
	}

	return &ListMessageEventsResult{
		Events:     events,
		NextCursor: cursor,
	}, nil
}

func (s *Service) ListRequestMessages(ctx context.Context, input ListRequestMessagesInput) (*ListRequestMessagesResult, error) {
	messages, _, err := s.messagesRead.List(ctx, ports.MessageListQuery{
		WorkspaceID:            input.WorkspaceID,
		TransactionalRequestID: input.RequestID,
		Limit:                  input.Limit,
	})
	if err != nil {
		return nil, err
	}

	return &ListRequestMessagesResult{
		Messages: messages,
	}, nil
}

func (s *Service) ListAttempts(ctx context.Context, input ListAttemptsInput) ([]domain.DeliveryAttempt, error) {
	return s.attemptsRead.ListByMessage(ctx, input.WorkspaceID, input.MessageID)
}

func (s *Service) ProcessDueMessages(ctx context.Context, input ProcessDueMessagesInput) (*ProcessDueMessagesResult, error) {
	return s.commands.ProcessDueMessages(ctx, input)
}
