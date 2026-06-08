package ports

import (
	"context"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/delivery/domain"
)

type MessageListQuery struct {
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

type DueMessageQuery struct {
	Now         time.Time
	Limit       int
	MessageType string
	WorkspaceID string
}

type MessageReadRepository interface {
	FindByID(ctx context.Context, workspaceID, messageID string) (*domain.Message, error)
	FindByIDForUpdate(ctx context.Context, workspaceID, messageID string) (*domain.Message, error)
	FindByTransactionalRequestID(ctx context.Context, workspaceID, transactionalRequestID string) (*domain.Message, error)
	FindByProviderMessageID(ctx context.Context, provider, providerMessageID string) (*domain.Message, error)
	List(ctx context.Context, query MessageListQuery) ([]domain.Message, string, error)
	ListDueQueued(ctx context.Context, query DueMessageQuery) ([]domain.Message, error)
	ListDistinctWorkspacesWithDue(ctx context.Context, messageType string, now time.Time) ([]string, error)
	CountByCampaign(ctx context.Context, workspaceID, campaignID string) (int64, error)
}

type MessageWriteRepository interface {
	CreateMany(ctx context.Context, messages []domain.Message) ([]string, error)
	Update(ctx context.Context, message domain.Message) error
	MarkProcessing(ctx context.Context, workspaceID, messageID string, now time.Time) error
	MarkAccepted(ctx context.Context, message domain.Message) error
	MarkDelivered(ctx context.Context, message domain.Message) error
	MarkBounced(ctx context.Context, message domain.Message) error
	MarkComplained(ctx context.Context, message domain.Message) error
	MarkDelayed(ctx context.Context, message domain.Message) error
	MarkFailed(ctx context.Context, message domain.Message) error
}

type AttemptReadRepository interface {
	ListByMessage(ctx context.Context, workspaceID, messageID string) ([]domain.DeliveryAttempt, error)
	NextAttemptNumber(ctx context.Context, workspaceID, messageID string) (int, error)
}

type AttemptWriteRepository interface {
	Create(ctx context.Context, attempt domain.DeliveryAttempt) error
	Update(ctx context.Context, attempt domain.DeliveryAttempt) error
}

type RetryStateReadRepository interface {
	FindByMessage(ctx context.Context, workspaceID, messageID string) (*domain.RetryState, error)
}

type RetryStateWriteRepository interface {
	Create(ctx context.Context, state domain.RetryState) error
	Update(ctx context.Context, state domain.RetryState) error
}

type TransactionalRequestReadRepository interface {
	FindByID(ctx context.Context, workspaceID, requestID string) (*domain.TransactionalSendRequest, error)
	FindByIdempotencyKey(ctx context.Context, workspaceID, idempotencyKey string) (*domain.TransactionalSendRequest, error)
}

type TransactionalRequestWriteRepository interface {
	Create(ctx context.Context, request domain.TransactionalSendRequest) error
	Update(ctx context.Context, request domain.TransactionalSendRequest) error
}
