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
	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/ports"
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
	TxManager           ports.UnitOfWork
	AccessChecker       ports.WorkspaceAccessChecker
	IDGen               func() (string, error)
	Logger              *slog.Logger
}

type Service struct {
	commands CommandBus
	queries  QueryBus
	log      *slog.Logger
}

func NewService(opts Options) *Service {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.IDGen == nil {
		opts.IDGen = id.NewUUIDGenerator().New
	}

	acceptTransactionalH := accepttransactionalsend.New(
		opts.TxRequestsRead,
		opts.TxRequestsWrite,
		opts.MessagesRead,
		opts.MessagesWrite,
		opts.SenderChecker,
		opts.ContentRenderer,
		opts.SuppressionChecker,
		opts.OutboxWriter,
		opts.TxManager,
		opts.IDGen,
		opts.Logger,
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
		opts.MessagesRead,
		opts.MessagesWrite,
		opts.RecipientSuppressor,
		opts.OutboxWriter,
		opts.TxManager,
		opts.IDGen,
		opts.Logger,
	)

	processDueMsgsH := processduemessages.New(
		opts.MessagesRead,
		opts.MessagesWrite,
		opts.AttemptsRead,
		opts.AttemptsWrite,
		opts.RetryStatesRead,
		opts.RetryStatesWrite,
		opts.SenderChecker,
		opts.SuppressionChecker,
		opts.ContentRenderer,
		opts.EmailProvider,
		opts.OutboxWriter,
		opts.TxManager,
		opts.IDGen,
		opts.Logger,
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
		log: opts.Logger.With("module", "delivery"),
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
	RecipientEmail    string
	RecipientName     string
	SenderDomainID    string
	TemplateID        string
	TemplateVersionID string
	TemplateData      map[string]any
	Metadata          map[string]any
	Tags              []string
	Now               time.Time
}

type AcceptTransactionalSendResult struct {
	MessageID  string
	RequestID  string
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

func (s *Service) ProcessDueMessages(ctx context.Context, input ProcessDueMessagesInput) (*ProcessDueMessagesResult, error) {
	return s.commands.ProcessDueMessages(ctx, input)
}
