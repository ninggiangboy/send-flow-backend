package app

import (
	"context"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/app/anomalies"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/app/campaign"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/app/dashboard"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/app/deliverability"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/app/forensics"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/app/ingestion"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/app/operationsanalytics"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/app/usage"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/domain"
	analyticsredis "github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/infrastructure/redis"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/ports"
)

type IngestEmailEventFactInput struct {
	SourceEventID     string
	SourceEventType   string
	WorkspaceID       string
	CampaignID        string
	MessageID         string
	Provider          string
	ProviderMessageID string
	ProviderEventID   string
	CanonicalType     string
	RecipientDomain   string
	OccurredAt        time.Time
	ReceivedAt        time.Time
	Metadata          map[string]any
}

type GetDashboardOverviewInput struct {
	WorkspaceID string
	UserID      string
}

type GetCampaignAnalyticsInput struct {
	WorkspaceID string
	CampaignID  string
	UserID      string
}

type GetDeliverabilityInput struct {
	WorkspaceID     string
	Provider        string
	RecipientDomain string
	UserID          string
}

type GetCampaignFunnelInput struct {
	WorkspaceID string
	CampaignID  string
	UserID      string
	From        time.Time
	To          time.Time
}

type GetCampaignTimeSeriesInput struct {
	WorkspaceID string
	CampaignID  string
	UserID      string
	From        time.Time
	To          time.Time
	Interval    string
	EventType   string
}

type GetCampaignBreakdownInput struct {
	WorkspaceID string
	CampaignID  string
	UserID      string
	From        time.Time
	To          time.Time
	GroupBy     string
}

type GetCampaignEventsInput struct {
	WorkspaceID string
	CampaignID  string
	UserID      string
	From        time.Time
	To          time.Time
	EventType   string
	Provider    string
	Domain      string
	Limit       int
	Cursor      string
}

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
	ingestionH           *ingestion.Handler
	dashboardH           *dashboard.Handler
	campaignH            *campaign.Handler
	deliverabilityH      *deliverability.Handler
	forensicsH           *forensics.Handler
	operationsAnalyticsH *operationsanalytics.Handler
	usageH               *usage.Handler
	anomaliesH           *anomalies.Handler
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
		operationsAnalyticsH: operationsanalytics.New(operationsanalytics.Options{
			OperationsQueryRepo: opts.OperationsQueryRepo,
			AccessChecker:       opts.AccessChecker,
			Logger:              opts.Logger,
		}),
		usageH: usage.New(usage.Options{
			UsageQueryRepo: opts.UsageQueryRepo,
			AccessChecker:  opts.AccessChecker,
			Logger:         opts.Logger,
		}),
		anomaliesH: anomalies.New(anomalies.Options{
			UsageQueryRepo:         opts.UsageQueryRepo,
			AnomalySignalWriteRepo: opts.AnomalySignalWriteRepo,
			AccessChecker:          opts.AccessChecker,
			Logger:                 opts.Logger,
			Clock:                  opts.Clock,
		}),
	}
}

// ── Facade methods ──

func (s *Service) IngestEmailEventFact(ctx context.Context, input IngestEmailEventFactInput) error {
	return s.ingestionH.ExecuteIngestEmailEventFact(ctx, ingestion.IngestFactCommand{
		SourceEventID:     input.SourceEventID,
		SourceEventType:   input.SourceEventType,
		WorkspaceID:       input.WorkspaceID,
		CampaignID:        input.CampaignID,
		MessageID:         input.MessageID,
		Provider:          input.Provider,
		ProviderMessageID: input.ProviderMessageID,
		ProviderEventID:   input.ProviderEventID,
		CanonicalType:     input.CanonicalType,
		RecipientDomain:   input.RecipientDomain,
		OccurredAt:        input.OccurredAt,
		ReceivedAt:        input.ReceivedAt,
		Metadata:          input.Metadata,
	})
}

type IngestOperationsEventInput struct {
	Source          string
	SourceEventID   string
	SourceEventType string
	OperationType   string
	Status          string
	WorkspaceID     string
	ErrorType       string
	Consumer        string
	Target          string
	Metadata        map[string]any
	OccurredAt      time.Time
}

func (s *Service) IngestOperationsEvent(ctx context.Context, input IngestOperationsEventInput) error {
	return s.ingestionH.ExecuteIngestOperationsEvent(ctx, ingestion.IngestOpsCommand{
		Source:          input.Source,
		SourceEventID:   input.SourceEventID,
		SourceEventType: input.SourceEventType,
		OperationType:   input.OperationType,
		Status:          input.Status,
		WorkspaceID:     input.WorkspaceID,
		ErrorType:       input.ErrorType,
		Consumer:        input.Consumer,
		Target:          input.Target,
		Metadata:        input.Metadata,
		OccurredAt:      input.OccurredAt,
	})
}

func (s *Service) GetDashboardOverview(ctx context.Context, input GetDashboardOverviewInput) (*domain.DashboardOverview, error) {
	return s.dashboardH.ExecuteGetDashboardOverview(ctx, dashboard.DashboardQuery{
		WorkspaceID: input.WorkspaceID,
		UserID:      input.UserID,
	})
}

func (s *Service) GetCampaignAnalytics(ctx context.Context, input GetCampaignAnalyticsInput) (*domain.CampaignAnalytics, error) {
	return s.campaignH.ExecuteCampaignAnalytics(ctx, campaign.CampaignAnalyticsQuery{
		WorkspaceID: input.WorkspaceID,
		CampaignID:  input.CampaignID,
		UserID:      input.UserID,
	})
}

func (s *Service) GetDeliverability(ctx context.Context, input GetDeliverabilityInput) (*domain.DeliverabilityResult, error) {
	return s.deliverabilityH.ExecuteGetDeliverability(ctx, deliverability.DeliverabilityQuery{
		WorkspaceID:     input.WorkspaceID,
		Provider:        input.Provider,
		RecipientDomain: input.RecipientDomain,
		UserID:          input.UserID,
	})
}

func (s *Service) GetCampaignFunnel(ctx context.Context, input GetCampaignFunnelInput) (*domain.CampaignFunnel, error) {
	return s.campaignH.ExecuteCampaignFunnel(ctx, campaign.CampaignFunnelQuery{
		WorkspaceID: input.WorkspaceID,
		CampaignID:  input.CampaignID,
		UserID:      input.UserID,
		From:        input.From,
		To:          input.To,
	})
}

func (s *Service) GetCampaignTimeSeries(ctx context.Context, input GetCampaignTimeSeriesInput) (*domain.CampaignTimeSeriesResult, error) {
	return s.campaignH.ExecuteCampaignTimeSeries(ctx, campaign.CampaignTimeSeriesQuery{
		WorkspaceID: input.WorkspaceID,
		CampaignID:  input.CampaignID,
		UserID:      input.UserID,
		From:        input.From,
		To:          input.To,
		Interval:    input.Interval,
		EventType:   input.EventType,
	})
}

func (s *Service) GetCampaignBreakdown(ctx context.Context, input GetCampaignBreakdownInput) (*domain.CampaignBreakdownResult, error) {
	return s.campaignH.ExecuteCampaignBreakdown(ctx, campaign.CampaignBreakdownQuery{
		WorkspaceID: input.WorkspaceID,
		CampaignID:  input.CampaignID,
		UserID:      input.UserID,
		From:        input.From,
		To:          input.To,
		GroupBy:     input.GroupBy,
	})
}

func (s *Service) GetCampaignEvents(ctx context.Context, input GetCampaignEventsInput) (*domain.CampaignEventsResult, error) {
	return s.campaignH.ExecuteCampaignEvents(ctx, campaign.CampaignEventsQuery{
		WorkspaceID: input.WorkspaceID,
		CampaignID:  input.CampaignID,
		UserID:      input.UserID,
		From:        input.From,
		To:          input.To,
		EventType:   input.EventType,
		Provider:    input.Provider,
		Domain:      input.Domain,
		Limit:       input.Limit,
		Cursor:      input.Cursor,
	})
}

type GetDeliverabilityTimeSeriesInput struct {
	WorkspaceID string
	UserID      string
	From        time.Time
	To          time.Time
	Provider    string
	Domain      string
	Interval    string
}

func (s *Service) GetDeliverabilityTimeSeries(ctx context.Context, input GetDeliverabilityTimeSeriesInput) (*domain.DeliverabilityTimeSeriesResult, error) {
	return s.deliverabilityH.ExecuteGetDeliverabilityTimeSeries(ctx, deliverability.TimeSeriesQuery{
		WorkspaceID: input.WorkspaceID,
		UserID:      input.UserID,
		From:        input.From,
		To:          input.To,
		Provider:    input.Provider,
		Domain:      input.Domain,
		Interval:    input.Interval,
	})
}

type GetDeliverabilityBreakdownInput struct {
	WorkspaceID string
	UserID      string
	From        time.Time
	To          time.Time
	GroupBy     string
	Provider    string
	Domain      string
}

func (s *Service) GetDeliverabilityBreakdown(ctx context.Context, input GetDeliverabilityBreakdownInput) (*domain.DeliverabilityBreakdownResult, error) {
	return s.deliverabilityH.ExecuteGetDeliverabilityBreakdown(ctx, deliverability.BreakdownQuery{
		WorkspaceID: input.WorkspaceID,
		UserID:      input.UserID,
		From:        input.From,
		To:          input.To,
		Provider:    input.Provider,
		Domain:      input.Domain,
		GroupBy:     input.GroupBy,
	})
}

type GetDeliverabilityLatencyInput struct {
	WorkspaceID string
	UserID      string
	From        time.Time
	To          time.Time
	Provider    string
	Domain      string
}

func (s *Service) GetDeliverabilityLatency(ctx context.Context, input GetDeliverabilityLatencyInput) (*domain.DeliverabilityLatencyResult, error) {
	return s.deliverabilityH.ExecuteGetDeliverabilityLatency(ctx, deliverability.LatencyQuery{
		WorkspaceID: input.WorkspaceID,
		UserID:      input.UserID,
		From:        input.From,
		To:          input.To,
		Provider:    input.Provider,
		Domain:      input.Domain,
	})
}

type GetDeliverabilityIncidentsInput struct {
	WorkspaceID string
	UserID      string
	From        time.Time
	To          time.Time
	Provider    string
	Domain      string
}

func (s *Service) GetDeliverabilityIncidents(ctx context.Context, input GetDeliverabilityIncidentsInput) (*domain.DeliverabilityIncidentResult, error) {
	return s.deliverabilityH.ExecuteGetDeliverabilityIncidents(ctx, deliverability.IncidentsQuery{
		WorkspaceID: input.WorkspaceID,
		UserID:      input.UserID,
		From:        input.From,
		To:          input.To,
		Provider:    input.Provider,
		Domain:      input.Domain,
	})
}

type SearchEventsInput struct {
	WorkspaceID string
	UserID      string
	From        time.Time
	To          time.Time
	EventType   string
	Provider    string
	Domain      string
	CampaignID  string
	MessageID   string
	Limit       int
	Cursor      string
}

func (s *Service) SearchEvents(ctx context.Context, input SearchEventsInput) (*domain.ForensicEventsResult, error) {
	return s.forensicsH.ExecuteSearchEvents(ctx, forensics.SearchQuery{
		WorkspaceID: input.WorkspaceID,
		UserID:      input.UserID,
		From:        input.From,
		To:          input.To,
		EventType:   input.EventType,
		Provider:    input.Provider,
		Domain:      input.Domain,
		CampaignID:  input.CampaignID,
		MessageID:   input.MessageID,
		Limit:       input.Limit,
		Cursor:      input.Cursor,
	})
}

type GetMessageTimelineInput struct {
	WorkspaceID string
	UserID      string
	MessageID   string
}

func (s *Service) GetMessageTimeline(ctx context.Context, input GetMessageTimelineInput) (*domain.MessageTimelineResult, error) {
	return s.forensicsH.ExecuteGetMessageTimeline(ctx, forensics.MessageTimelineQuery{
		WorkspaceID: input.WorkspaceID,
		MessageID:   input.MessageID,
		UserID:      input.UserID,
	})
}

type GetProviderEventTraceInput struct {
	WorkspaceID     string
	UserID          string
	ProviderEventID string
}

func (s *Service) GetProviderEventTrace(ctx context.Context, input GetProviderEventTraceInput) (*domain.ProviderEventTrace, error) {
	return s.forensicsH.ExecuteGetProviderEventTrace(ctx, forensics.ProviderEventTraceQuery{
		WorkspaceID:     input.WorkspaceID,
		ProviderEventID: input.ProviderEventID,
		UserID:          input.UserID,
	})
}

type GetCampaignIncidentTimelineInput struct {
	WorkspaceID string
	CampaignID  string
	UserID      string
	From        time.Time
	To          time.Time
}

func (s *Service) GetCampaignIncidentTimeline(ctx context.Context, input GetCampaignIncidentTimelineInput) (*domain.CampaignIncidentTimelineResult, error) {
	return s.forensicsH.ExecuteGetCampaignIncidentTimeline(ctx, forensics.IncidentTimelineQuery{
		WorkspaceID: input.WorkspaceID,
		CampaignID:  input.CampaignID,
		UserID:      input.UserID,
		From:        input.From,
		To:          input.To,
	})
}

type GetOutboxLagInput struct {
	WorkspaceID string
	UserID      string
	From        time.Time
	To          time.Time
	Source      string
}

func (s *Service) GetOutboxLag(ctx context.Context, input GetOutboxLagInput) (*domain.OutboxLagResult, error) {
	return s.operationsAnalyticsH.ExecuteGetOutboxLag(ctx, operationsanalytics.OutboxLagQuery{
		WorkspaceID: input.WorkspaceID,
		UserID:      input.UserID,
		From:        input.From,
		To:          input.To,
		Source:      input.Source,
	})
}

type GetConsumerFailuresInput struct {
	WorkspaceID string
	UserID      string
	From        time.Time
	To          time.Time
	Source      string
}

func (s *Service) GetConsumerFailures(ctx context.Context, input GetConsumerFailuresInput) (*domain.ConsumerFailureResult, error) {
	return s.operationsAnalyticsH.ExecuteGetConsumerFailures(ctx, operationsanalytics.ConsumerFailuresQuery{
		WorkspaceID: input.WorkspaceID,
		UserID:      input.UserID,
		From:        input.From,
		To:          input.To,
		Source:      input.Source,
	})
}

type GetDLQVolumeInput struct {
	WorkspaceID string
	UserID      string
	From        time.Time
	To          time.Time
	Source      string
}

func (s *Service) GetDLQVolume(ctx context.Context, input GetDLQVolumeInput) (*domain.DLQResult, error) {
	return s.operationsAnalyticsH.ExecuteGetDLQVolume(ctx, operationsanalytics.DLQVolumeQuery{
		WorkspaceID: input.WorkspaceID,
		UserID:      input.UserID,
		From:        input.From,
		To:          input.To,
		Source:      input.Source,
	})
}

type GetWebhookDeliveryTimeSeriesInput struct {
	WorkspaceID string
	UserID      string
	From        time.Time
	To          time.Time
	Status      string
	Interval    string
}

func (s *Service) GetWebhookDeliveryTimeSeries(ctx context.Context, input GetWebhookDeliveryTimeSeriesInput) (*domain.WebhookDeliveryTimeSeriesResult, error) {
	return s.operationsAnalyticsH.ExecuteGetWebhookDeliveryTimeSeries(ctx, operationsanalytics.WebhookDeliveryTimeSeriesQuery{
		WorkspaceID: input.WorkspaceID,
		UserID:      input.UserID,
		From:        input.From,
		To:          input.To,
		Status:      input.Status,
		Interval:    input.Interval,
	})
}

type GetWebhookReliabilityInput struct {
	WorkspaceID string
	UserID      string
	From        time.Time
	To          time.Time
	Target      string
}

func (s *Service) GetWebhookReliability(ctx context.Context, input GetWebhookReliabilityInput) (*domain.WebhookReliabilityResult, error) {
	return s.operationsAnalyticsH.ExecuteGetWebhookReliability(ctx, operationsanalytics.WebhookReliabilityQuery{
		WorkspaceID: input.WorkspaceID,
		UserID:      input.UserID,
		From:        input.From,
		To:          input.To,
		Target:      input.Target,
	})
}

type GetUsageTimeSeriesInput struct {
	WorkspaceID string
	UserID      string
	From        time.Time
	To          time.Time
	Interval    string
}

func (s *Service) GetUsageTimeSeries(ctx context.Context, input GetUsageTimeSeriesInput) (*domain.UsageTimeSeriesResult, error) {
	return s.usageH.ExecuteGetUsageTimeSeries(ctx, usage.UsageTimeSeriesQuery{
		WorkspaceID: input.WorkspaceID,
		UserID:      input.UserID,
		From:        input.From,
		To:          input.To,
		Interval:    input.Interval,
	})
}

type GetUsageFeaturesInput struct {
	WorkspaceID string
	UserID      string
	From        time.Time
	To          time.Time
}

func (s *Service) GetUsageFeatures(ctx context.Context, input GetUsageFeaturesInput) (*domain.UsageFeaturesResult, error) {
	return s.usageH.ExecuteGetUsageFeatures(ctx, usage.UsageFeaturesQuery{
		WorkspaceID: input.WorkspaceID,
		UserID:      input.UserID,
		From:        input.From,
		To:          input.To,
	})
}

type GetRiskSignalsInput struct {
	WorkspaceID string
	UserID      string
	From        time.Time
	To          time.Time
}

func (s *Service) GetRiskSignals(ctx context.Context, input GetRiskSignalsInput) (*domain.RiskSignalsResult, error) {
	return s.usageH.ExecuteGetRiskSignals(ctx, usage.RiskSignalsQuery{
		WorkspaceID: input.WorkspaceID,
		UserID:      input.UserID,
		From:        input.From,
		To:          input.To,
	})
}

type GetSendVolumeForecastInput struct {
	WorkspaceID string
	UserID      string
	From        time.Time
	To          time.Time
}

func (s *Service) GetSendVolumeForecast(ctx context.Context, input GetSendVolumeForecastInput) (*domain.SendVolumeForecastResult, error) {
	return s.usageH.ExecuteGetSendVolumeForecast(ctx, usage.SendVolumeForecastQuery{
		WorkspaceID: input.WorkspaceID,
		UserID:      input.UserID,
		From:        input.From,
		To:          input.To,
	})
}

type GetAnomaliesInput struct {
	WorkspaceID string
	UserID      string
	From        time.Time
	To          time.Time
}

func (s *Service) GetAnomalies(ctx context.Context, input GetAnomaliesInput) (*domain.AnomaliesResult, error) {
	return s.anomaliesH.ExecuteGetAnomalies(ctx, anomalies.AnomaliesQuery{
		WorkspaceID: input.WorkspaceID,
		UserID:      input.UserID,
		From:        input.From,
		To:          input.To,
	})
}

func (s *Service) DetectAnomaliesAllWorkspaces(ctx context.Context) error {
	return s.anomaliesH.ExecuteDetectAnomaliesAllWorkspaces(ctx)
}
