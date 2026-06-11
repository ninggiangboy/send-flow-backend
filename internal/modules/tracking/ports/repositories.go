package ports

import (
	"context"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/tracking/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/outbox"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/transaction"
)

type TrackingLinkReadRepository interface {
	FindByID(ctx context.Context, trackingID string) (*domain.TrackingLink, error)
	ListByMessage(ctx context.Context, workspaceID, messageID string) ([]domain.TrackingLink, error)
}

type TrackingLinkWriteRepository interface {
	Create(ctx context.Context, link domain.TrackingLink) error
}

type TrackingEventReadRepository interface {
	FindBySourceEvent(ctx context.Context, source, sourceEventID, eventType string) (*domain.TrackingEvent, error)
	ListByMessage(ctx context.Context, workspaceID, messageID string, limit int, cursor string) ([]domain.TrackingEvent, string, error)
}

type TrackingEventWriteRepository interface {
	Create(ctx context.Context, event domain.TrackingEvent) error
}

type DeliveryMessageResolver interface {
	FindByID(ctx context.Context, workspaceID, messageID string) (messageIDOut string, workspaceIDOut string, campaignID string, provider string, providerMessageID string, recipientEmailNormalized string, err error)
	FindByProviderMessageID(ctx context.Context, provider, providerMessageID string) (messageIDOut string, workspaceIDOut string, campaignID string, err error)
}

type OutboxEvent = outbox.Event

type OutboxWriter interface {
	Save(ctx context.Context, event OutboxEvent) error
}

type TransactionManager = transaction.UnitOfWork
