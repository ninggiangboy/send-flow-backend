package app

import (
	"context"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/app/accepttransactionalsend"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/app/getmessage"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/app/gettransactionalmessage"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/app/handlecampaignscheduled"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/app/handleproviderevent"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/app/listmessages"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/app/processduemessages"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/app/queuecampaignmessages"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/app/usecase"
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
	acceptTransactionalH := accepttransactionalsend.New(
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

	queueCampaignH := queuecampaignmessages.New(
		opts.CampaignReader,
		opts.MessagesWrite,
		opts.OutboxWriter,
		opts.TxManager,
		opts.IDGen,
		opts.Logger,
	)

	handleCampaignH := handlecampaignscheduled.New(queueCampaignH, opts.Logger)

	handleProviderH := handleproviderevent.New(
		opts.MessagesWrite,
		opts.TxRequestsWrite,
		opts.RecipientSuppressor,
		opts.OutboxWriter,
		opts.TxManager,
		opts.IDGen,
		opts.Logger,
		opts.EventRepo,
	)

	processDueMsgsH := processduemessages.New(
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

	listMessagesH := listmessages.New(opts.MessagesRead, opts.AccessChecker, opts.Logger)
	getMessageH := getmessage.New(opts.MessagesRead, opts.AccessChecker, opts.Logger)
	getTransactionalH := gettransactionalmessage.New(opts.MessagesRead, opts.Logger)

	return &Service{
		commands: newCommandBus(opts.Logger,
			acceptTransactionalH,
			queueCampaignH,
			handleCampaignH,
			handleProviderH,
			processDueMsgsH,
		),
		queries: newQueryBus(opts.Logger,
			listMessagesH,
			getMessageH,
			getTransactionalH,
		),
		log:          opts.Logger.With("module", "delivery"),
		eventRepo:    opts.EventRepo,
		messagesRead: opts.MessagesRead,
		attemptsRead: opts.AttemptsRead,
	}
}

// Shared types used by external callers

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

type ListMessagesInput struct {
	UserID                   string
	WorkspaceID              string
	CampaignID               string
	TransactionalRequestID   string
	Status                   string
	MessageType              string
	Mode                     string
	Provider                 string
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

type GetMessageInput struct {
	UserID      string
	WorkspaceID string
	MessageID   string
}

type AcceptTransactionalSendInput struct {
	WorkspaceID       string
	APIKeyID          string
	IdempotencyKey    string
	Mode              string
	SenderDomainID    string
	SenderName        string
	Subject           string
	TemplateID        string
	TemplateVersionID string
	TemplateData      map[string]any
	TextBody          string
	HTMLBody          string
	ReplyTo           string
	To                []domain.RecipientTarget
	CC                []domain.RecipientTarget
	BCC               []domain.RecipientTarget
	Metadata          map[string]any
	Tags              []string
	Headers           map[string]string
	Attachments       []accepttransactionalsend.AttachmentStream
	Now               time.Time
}

type AcceptTransactionalSendResult struct {
	RequestID  string
	MessageIDs []string
	Status     string
	AcceptedAt time.Time
}

type GetTransactionalMessageInput struct {
	WorkspaceID string
	MessageID   string
}

type GetTransactionalMessageResult struct {
	MessageID         string     `json:"message_id"`
	Status            string     `json:"status"`
	Provider          string     `json:"provider,omitempty"`
	ProviderMessageID string     `json:"provider_message_id,omitempty"`
	LastUpdatedAt     *time.Time `json:"last_updated_at"`
}

type HandleCampaignScheduledInput struct {
	EventID   string
	EventType string
	Payload   []byte
	Now       time.Time
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
	Handled            bool
	Ignored            bool
	MessageID          string
	WorkspaceID        string
	PreviousStatus     string
	NewStatus          string
	SuppressionCreated bool
	SuppressionEntryID string
}

type ListMessageEventsInput struct {
	WorkspaceID string
	MessageID   string
	Limit       int
	Cursor      string
}

type ListMessageEventsResult struct {
	Events     []domain.MessageEvent
	NextCursor string
}

type ListRequestMessagesInput struct {
	WorkspaceID string
	RequestID   string
	Limit       int
	Cursor      string
}

type ListRequestMessagesResult struct {
	Messages []domain.Message
}

type ListAttemptsInput struct {
	WorkspaceID string
	MessageID   string
}

type ListAttemptsResult struct {
	Attempts []domain.DeliveryAttempt
}

type ProcessDueMessagesAllInput struct {
	MessageType string
	Limit       int
	Now         time.Time
}

type ProcessDueMessagesInput struct {
	WorkspaceID string
	MessageType string
	Limit       int
	Now         time.Time
}

type ProcessDueMessagesResult struct {
	SelectedCount       int
	AcceptedCount       int
	FailedCount         int
	RetryScheduledCount int
}

type (
	NonRetryableError        = usecase.NonRetryableError
	RecipientSuppressor      = handleproviderevent.RecipientSuppressor
	SuppressFromSignalInput  = handleproviderevent.SuppressFromSignalInput
	SuppressFromSignalResult = handleproviderevent.SuppressFromSignalResult
)

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
