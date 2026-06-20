package app

import (
	"context"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/app/anomaly"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/app/campaign"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/app/dashboard"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/app/deliverability"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/app/forensics"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/app/ingestion"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/app/operations"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/app/usage"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/domain"
	analyticsredis "github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/infrastructure/redis"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/ports"
)

type Options struct {
	FactRepo                ports.EventFactRepository
	WorkspaceQueryRepo      ports.WorkspaceQueryRepository
	CampaignQueryRepo       ports.CampaignQueryRepository
	DeliverabilityQueryRepo ports.DeliverabilityQueryRepository
	ForensicQueryRepo       ports.ForensicQueryRepository
	OperationsQueryRepo     ports.OperationsQueryRepository
	UsageQueryRepo          ports.UsageQueryRepository
	AnomalySignalWriteRepo  ports.AnomalySignalWriteRepository
	OperationsEventWriter   ports.OperationsEventWriter
	AccessChecker           ports.WorkspaceAccessChecker
	Clock                   func() time.Time
	Logger                  *slog.Logger
	RedisCache              *analyticsredis.Cache
}

type Service struct {
	ingestionH      *ingestion.Handler
	dashboardH      *dashboard.Handler
	campaignH       *campaign.Handler
	deliverabilityH *deliverability.Handler
	forensicsH      *forensics.Handler
	operationsH     *operations.Handler
	usageH          *usage.Handler
	anomalyH        *anomaly.Handler
}

func NewService(opts Options) *Service {
	if opts.Clock == nil {
		opts.Clock = time.Now
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	return &Service{
		ingestionH: ingestion.New(ingestion.Options{
			FactRepo:              opts.FactRepo,
			Clock:                 opts.Clock,
			AccessChecker:         opts.AccessChecker,
			OperationsEventWriter: opts.OperationsEventWriter,
			Logger:                opts.Logger,
		}),
		dashboardH: dashboard.New(dashboard.Options{
			WorkspaceQueryRepo: opts.WorkspaceQueryRepo,
			AccessChecker:      opts.AccessChecker,
			Logger:             opts.Logger,
			Cache:              opts.RedisCache,
		}),
		campaignH: campaign.New(campaign.Options{
			CampaignQueryRepo: opts.CampaignQueryRepo,
			AccessChecker:     opts.AccessChecker,
			Logger:            opts.Logger,
			Cache:             opts.RedisCache,
		}),
		deliverabilityH: deliverability.New(deliverability.Options{
			DeliverabilityQueryRepo: opts.DeliverabilityQueryRepo,
			AccessChecker:           opts.AccessChecker,
			Logger:                  opts.Logger,
			Cache:                   opts.RedisCache,
		}),
		forensicsH: forensics.New(forensics.Options{
			ForensicQueryRepo: opts.ForensicQueryRepo,
			AccessChecker:     opts.AccessChecker,
			Logger:            opts.Logger,
		}),
		operationsH: operations.New(operations.Options{
			OperationsQueryRepo: opts.OperationsQueryRepo,
			AccessChecker:       opts.AccessChecker,
			Logger:              opts.Logger,
		}),
		usageH: usage.New(usage.Options{
			UsageQueryRepo: opts.UsageQueryRepo,
			AccessChecker:  opts.AccessChecker,
			Logger:         opts.Logger,
		}),
		anomalyH: anomaly.New(anomaly.Options{
			UsageQueryRepo:         opts.UsageQueryRepo,
			AnomalySignalWriteRepo: opts.AnomalySignalWriteRepo,
			AccessChecker:          opts.AccessChecker,
			Logger:                 opts.Logger,
			Clock:                  opts.Clock,
		}),
	}
}

// ── Facade methods ──
// Input types are aliases for grouped-package types (see types.go),
// enabling zero-copy pass-through with no field-by-field mapping.

func (s *Service) IngestEmailEventFact(ctx context.Context, input IngestEmailEventFactInput) error {
	return s.ingestionH.ExecuteIngestEmailEventFact(ctx, input)
}

func (s *Service) IngestOperationsEvent(ctx context.Context, input IngestOperationsEventInput) error {
	return s.ingestionH.ExecuteIngestOperationsEvent(ctx, input)
}

func (s *Service) GetDashboardOverview(ctx context.Context, input GetDashboardOverviewInput) (*domain.DashboardOverview, error) {
	return s.dashboardH.ExecuteGetDashboardOverview(ctx, input)
}

func (s *Service) GetCampaignAnalytics(ctx context.Context, input GetCampaignAnalyticsInput) (*domain.CampaignAnalytics, error) {
	return s.campaignH.ExecuteCampaignAnalytics(ctx, input)
}

func (s *Service) GetCampaignFunnel(ctx context.Context, input GetCampaignFunnelInput) (*domain.CampaignFunnel, error) {
	return s.campaignH.ExecuteCampaignFunnel(ctx, input)
}

func (s *Service) GetCampaignTimeSeries(ctx context.Context, input GetCampaignTimeSeriesInput) (*domain.CampaignTimeSeriesResult, error) {
	return s.campaignH.ExecuteCampaignTimeSeries(ctx, input)
}

func (s *Service) GetCampaignBreakdown(ctx context.Context, input GetCampaignBreakdownInput) (*domain.CampaignBreakdownResult, error) {
	return s.campaignH.ExecuteCampaignBreakdown(ctx, input)
}

func (s *Service) GetCampaignEvents(ctx context.Context, input GetCampaignEventsInput) (*domain.CampaignEventsResult, error) {
	return s.campaignH.ExecuteCampaignEvents(ctx, input)
}

func (s *Service) GetDeliverability(ctx context.Context, input GetDeliverabilityInput) (*domain.DeliverabilityResult, error) {
	return s.deliverabilityH.ExecuteGetDeliverability(ctx, input)
}

func (s *Service) GetDeliverabilityTimeSeries(ctx context.Context, input GetDeliverabilityTimeSeriesInput) (*domain.DeliverabilityTimeSeriesResult, error) {
	return s.deliverabilityH.ExecuteGetDeliverabilityTimeSeries(ctx, input)
}

func (s *Service) GetDeliverabilityBreakdown(ctx context.Context, input GetDeliverabilityBreakdownInput) (*domain.DeliverabilityBreakdownResult, error) {
	return s.deliverabilityH.ExecuteGetDeliverabilityBreakdown(ctx, input)
}

func (s *Service) GetDeliverabilityLatency(ctx context.Context, input GetDeliverabilityLatencyInput) (*domain.DeliverabilityLatencyResult, error) {
	return s.deliverabilityH.ExecuteGetDeliverabilityLatency(ctx, input)
}

func (s *Service) GetDeliverabilityIncidents(ctx context.Context, input GetDeliverabilityIncidentsInput) (*domain.DeliverabilityIncidentResult, error) {
	return s.deliverabilityH.ExecuteGetDeliverabilityIncidents(ctx, input)
}

func (s *Service) SearchEvents(ctx context.Context, input SearchEventsInput) (*domain.ForensicEventsResult, error) {
	return s.forensicsH.ExecuteSearchEvents(ctx, input)
}

func (s *Service) GetMessageTimeline(ctx context.Context, input GetMessageTimelineInput) (*domain.MessageTimelineResult, error) {
	return s.forensicsH.ExecuteGetMessageTimeline(ctx, input)
}

func (s *Service) GetProviderEventTrace(ctx context.Context, input GetProviderEventTraceInput) (*domain.ProviderEventTrace, error) {
	return s.forensicsH.ExecuteGetProviderEventTrace(ctx, input)
}

func (s *Service) GetCampaignIncidentTimeline(ctx context.Context, input GetCampaignIncidentTimelineInput) (*domain.CampaignIncidentTimelineResult, error) {
	return s.forensicsH.ExecuteGetCampaignIncidentTimeline(ctx, input)
}

func (s *Service) GetOutboxLag(ctx context.Context, input GetOutboxLagInput) (*domain.OutboxLagResult, error) {
	return s.operationsH.ExecuteGetOutboxLag(ctx, input)
}

func (s *Service) GetConsumerFailures(ctx context.Context, input GetConsumerFailuresInput) (*domain.ConsumerFailureResult, error) {
	return s.operationsH.ExecuteGetConsumerFailures(ctx, input)
}

func (s *Service) GetDLQVolume(ctx context.Context, input GetDLQVolumeInput) (*domain.DLQResult, error) {
	return s.operationsH.ExecuteGetDLQVolume(ctx, input)
}

func (s *Service) GetWebhookDeliveryTimeSeries(ctx context.Context, input GetWebhookDeliveryTimeSeriesInput) (*domain.WebhookDeliveryTimeSeriesResult, error) {
	return s.operationsH.ExecuteGetWebhookDeliveryTimeSeries(ctx, input)
}

func (s *Service) GetWebhookReliability(ctx context.Context, input GetWebhookReliabilityInput) (*domain.WebhookReliabilityResult, error) {
	return s.operationsH.ExecuteGetWebhookReliability(ctx, input)
}

func (s *Service) GetUsageTimeSeries(ctx context.Context, input GetUsageTimeSeriesInput) (*domain.UsageTimeSeriesResult, error) {
	return s.usageH.ExecuteGetUsageTimeSeries(ctx, input)
}

func (s *Service) GetUsageFeatures(ctx context.Context, input GetUsageFeaturesInput) (*domain.UsageFeaturesResult, error) {
	return s.usageH.ExecuteGetUsageFeatures(ctx, input)
}

func (s *Service) GetRiskSignals(ctx context.Context, input GetRiskSignalsInput) (*domain.RiskSignalsResult, error) {
	return s.usageH.ExecuteGetRiskSignals(ctx, input)
}

func (s *Service) GetSendVolumeForecast(ctx context.Context, input GetSendVolumeForecastInput) (*domain.SendVolumeForecastResult, error) {
	return s.usageH.ExecuteGetSendVolumeForecast(ctx, input)
}

func (s *Service) GetAnomalies(ctx context.Context, input GetAnomaliesInput) (*domain.AnomaliesResult, error) {
	return s.anomalyH.ExecuteGetAnomalies(ctx, input)
}

func (s *Service) DetectAnomaliesAllWorkspaces(ctx context.Context) error {
	return s.anomalyH.ExecuteDetectAnomaliesAllWorkspaces(ctx)
}
