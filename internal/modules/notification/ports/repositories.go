package ports

import (
	"context"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/notification/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/auth"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/outbox"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/transaction"
)

type MessageReadRepository interface {
	FindByID(ctx context.Context, id string) (*domain.NotificationMessage, error)
	List(ctx context.Context, filter domain.NotificationFilter) ([]domain.NotificationMessage, string, error)
	FindPendingForRetry(ctx context.Context, limit int) ([]domain.NotificationMessage, error)
}

type MessageWriteRepository interface {
	Create(ctx context.Context, msg domain.NotificationMessage) error
	Update(ctx context.Context, msg domain.NotificationMessage) error
	ClaimRetryingMessages(ctx context.Context, limit int) ([]domain.NotificationMessage, error)
}

type AttemptReadRepository interface {
	FindByMessageID(ctx context.Context, notificationMessageID string) ([]domain.NotificationAttempt, error)
}

type AttemptWriteRepository interface {
	Create(ctx context.Context, attempt domain.NotificationAttempt) error
}

type OutboxEvent = outbox.Event

type OutboxWriter interface {
	Save(ctx context.Context, event OutboxEvent) error
}

type TransactionManager = transaction.UnitOfWork

type WorkspaceAccessChecker = auth.WorkspaceAccessChecker

type EmailSender interface {
	SendNotificationEmail(ctx context.Context, to []string, subject, textBody, htmlBody string) error
}
