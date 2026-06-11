package ports

import (
	"context"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/auth"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/outbox"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/transaction"
)

type WorkspaceAccessChecker = auth.WorkspaceAccessChecker

type OutboxEvent = outbox.Event

type EventFactRepository interface {
	Create(ctx context.Context, fact domain.EmailEventFact) error
	FindBySourceEventID(ctx context.Context, sourceEventID string) (*domain.EmailEventFact, error)
}

type ProjectionReadRepository interface {
	GetWorkspaceOverview(ctx context.Context, workspaceID string) (*domain.WorkspaceAnalyticsOverview, error)
	GetCampaignSummary(ctx context.Context, workspaceID, campaignID string) (*domain.CampaignDeliverySummary, error)
	ListDeliverability(ctx context.Context, workspaceID string, filter domain.DeliverabilityFilter) ([]domain.DeliverabilityProjection, error)
}

type ProjectionWriteRepository interface {
	IncrementWorkspaceOverview(ctx context.Context, workspaceID string, eventType string, occurredAt time.Time) error
	IncrementCampaignSummary(ctx context.Context, workspaceID, campaignID string, eventType string, occurredAt time.Time) error
	IncrementDeliverability(ctx context.Context, workspaceID, provider, recipientDomain, eventType string, occurredAt time.Time) error
}

type ProjectionRepository interface {
	ProjectionReadRepository
	ProjectionWriteRepository
}

type TransactionManager = transaction.UnitOfWork

type OutboxWriter interface {
	Save(ctx context.Context, event OutboxEvent) error
}
