package ports

import (
	"context"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/domain"
)

type WorkspaceAccessChecker interface {
	RequirePermission(ctx context.Context, workspaceID, userID, permission string) error
}

type OutboxEvent struct {
	ID            string
	AggregateType string
	AggregateID   string
	EventType     string
	Payload       []byte
	WorkspaceID   string
	OccurredAt    time.Time
}

type EventFactRepository interface {
	Create(ctx context.Context, fact domain.EmailEventFact) error
	FindBySourceEventID(ctx context.Context, sourceEventID string) (*domain.EmailEventFact, error)
}

type ProjectionRepository interface {
	IncrementWorkspaceOverview(ctx context.Context, workspaceID string, eventType string, occurredAt time.Time) error
	IncrementCampaignSummary(ctx context.Context, workspaceID, campaignID string, eventType string, occurredAt time.Time) error
	IncrementDeliverability(ctx context.Context, workspaceID, provider, recipientDomain, eventType string, occurredAt time.Time) error
	GetWorkspaceOverview(ctx context.Context, workspaceID string) (*domain.WorkspaceAnalyticsOverview, error)
	GetCampaignSummary(ctx context.Context, workspaceID, campaignID string) (*domain.CampaignDeliverySummary, error)
	ListDeliverability(ctx context.Context, workspaceID string, filter domain.DeliverabilityFilter) ([]domain.DeliverabilityProjection, error)
}

type TransactionManager interface {
	RunInTransaction(ctx context.Context, fn func(context.Context) error) error
}

type OutboxWriter interface {
	Save(ctx context.Context, event OutboxEvent) error
}
