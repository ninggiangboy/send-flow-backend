package ports

import (
	"context"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/webhooks/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/auth"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/outbox"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/transaction"
)

type ConfigReadRepository interface {
	FindByID(ctx context.Context, workspaceID, webhookID string) (*domain.WebhookConfig, error)
	ListByWorkspace(ctx context.Context, workspaceID string) ([]domain.WebhookConfig, error)
	ListSubscribed(ctx context.Context, workspaceID, eventType string) ([]domain.WebhookConfig, error)
}

type ConfigWriteRepository interface {
	ConfigReadRepository
	Create(ctx context.Context, config domain.WebhookConfig) error
	Update(ctx context.Context, config domain.WebhookConfig) error
	Disable(ctx context.Context, workspaceID, webhookID string, disabledAt time.Time) error
}

type DeliveryFilter struct {
	WebhookID string
	Status    string
	EventType string
	From      *time.Time
	To        *time.Time
	Limit     int
	Cursor    string
}

type DeliveryReadRepository interface {
	FindByID(ctx context.Context, workspaceID, deliveryID string) (*domain.WebhookDelivery, error)
	FindByWebhookAndEvent(ctx context.Context, webhookID, sourceEventID string) (*domain.WebhookDelivery, error)
	ListByWorkspace(ctx context.Context, workspaceID string, filter DeliveryFilter) ([]domain.WebhookDelivery, string, error)
}

type DeliveryWriteRepository interface {
	DeliveryReadRepository
	Create(ctx context.Context, delivery domain.WebhookDelivery) error
	MarkDelivering(ctx context.Context, deliveryID string, now time.Time) error
	MarkSucceeded(ctx context.Context, deliveryID string, result domain.DeliveryResult) error
	MarkFailed(ctx context.Context, deliveryID string, result domain.DeliveryResult) error
	ScheduleRetry(ctx context.Context, deliveryID string, nextAttemptAt time.Time) error
	ClaimPendingDeliveries(ctx context.Context, limit int, now time.Time) ([]domain.WebhookDelivery, error)
}

type AttemptReadRepository interface {
	ListByDelivery(ctx context.Context, deliveryID string) ([]domain.WebhookDeliveryAttempt, error)
}

type AttemptWriteRepository interface {
	AttemptReadRepository
	Create(ctx context.Context, attempt domain.WebhookDeliveryAttempt) error
}

type DeliveryHTTPRequest struct {
	URL             string
	Body            []byte
	SignatureHeader string
	SignatureValue  string
	TimestampHeader string
	TimestampValue  string
	EventIDHeader   string
	EventIDValue    string
}

type DeliveryHTTPResponse struct {
	StatusCode int
	Headers    map[string]string
	Body       string
	DurationMs int64
	Error      string
}

type HTTPDeliverer interface {
	Deliver(ctx context.Context, req DeliveryHTTPRequest) (DeliveryHTTPResponse, error)
}

type WorkspaceAccessChecker = auth.WorkspaceAccessChecker

type TransactionManager = transaction.UnitOfWork

type OutboxEvent = outbox.Event

type OutboxWriter interface {
	Save(ctx context.Context, event OutboxEvent) error
}
