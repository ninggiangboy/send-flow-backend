package ports

import (
	"context"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/auth"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/outbox"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/transaction"
)

type CampaignQueryRepository interface {
	GetCampaignFunnel(ctx context.Context, workspaceID, campaignID string, from, to time.Time) (*domain.CampaignFunnel, error)
	GetCampaignTimeSeries(ctx context.Context, workspaceID, campaignID string, from, to time.Time, interval, eventType string) (*domain.CampaignTimeSeriesResult, error)
	GetCampaignBreakdown(ctx context.Context, workspaceID, campaignID string, from, to time.Time, groupBy string) (*domain.CampaignBreakdownResult, error)
	GetCampaignEvents(ctx context.Context, f domain.CampaignQueryFilter) (*domain.CampaignEventsResult, error)
}

type DeliverabilityQueryRepository interface {
	GetDeliverabilityTimeSeries(ctx context.Context, workspaceID string, from, to time.Time, provider, recipientDomain, interval string) (*domain.DeliverabilityTimeSeriesResult, error)
	GetDeliverabilityBreakdown(ctx context.Context, workspaceID string, from, to time.Time, groupBy, provider, recipientDomain string) (*domain.DeliverabilityBreakdownResult, error)
	GetDeliverabilityLatency(ctx context.Context, workspaceID string, from, to time.Time, provider, recipientDomain string) (*domain.DeliverabilityLatencyResult, error)
	GetDeliverabilityIncidents(ctx context.Context, workspaceID string, from, to time.Time, provider, recipientDomain string) (*domain.DeliverabilityIncidentResult, error)
}

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

type ForensicQueryRepository interface {
	SearchEvents(ctx context.Context, f domain.ForensicQueryFilter) (*domain.ForensicEventsResult, error)
	GetMessageTimeline(ctx context.Context, workspaceID, messageID string) (*domain.MessageTimelineResult, error)
	GetProviderEventTrace(ctx context.Context, workspaceID, providerEventID string) (*domain.ProviderEventTrace, error)
	GetCampaignIncidentTimeline(ctx context.Context, workspaceID, campaignID string, from, to time.Time) (*domain.CampaignIncidentTimelineResult, error)
}

type OperationsQueryRepository interface {
	GetOutboxLag(ctx context.Context, workspaceID string, from, to time.Time, source string) (*domain.OutboxLagResult, error)
	GetConsumerFailures(ctx context.Context, workspaceID string, from, to time.Time, source string) (*domain.ConsumerFailureResult, error)
	GetDLQVolume(ctx context.Context, workspaceID string, from, to time.Time, source string) (*domain.DLQResult, error)
	GetWebhookDeliveryTimeSeries(ctx context.Context, workspaceID string, from, to time.Time, status string, interval string) (*domain.WebhookDeliveryTimeSeriesResult, error)
	GetWebhookReliability(ctx context.Context, workspaceID string, from, to time.Time, target string) (*domain.WebhookReliabilityResult, error)
}

type UsageQueryRepository interface {
	GetUsageTimeSeries(ctx context.Context, workspaceID string, from, to time.Time, interval string) (*domain.UsageTimeSeriesResult, error)
	GetUsageFeatures(ctx context.Context, workspaceID string, from, to time.Time) (*domain.UsageFeaturesResult, error)
	GetRiskSignals(ctx context.Context, workspaceID string, from, to time.Time) (*domain.RiskSignalsResult, error)
	GetSendVolumeForecast(ctx context.Context, workspaceID string, from, to time.Time) (*domain.SendVolumeForecastResult, error)
	GetAnomalies(ctx context.Context, workspaceID string, from, to time.Time) (*domain.AnomaliesResult, error)
	ListDistinctWorkspaces(ctx context.Context, since time.Time) ([]string, error)
}

type AnomalySignalWriteRepository interface {
	SaveAnomalySignals(ctx context.Context, workspaceID string, signals []domain.AnomalyRow) error
}

type OutboxWriter interface {
	Save(ctx context.Context, event OutboxEvent) error
}
