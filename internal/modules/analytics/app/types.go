package app

import (
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/app/anomaly"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/app/campaign"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/app/dashboard"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/app/deliverability"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/app/forensics"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/app/ingestion"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/app/operations"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/app/shared"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/analytics/app/usage"
)

// ── Mapper re-exports (moved to shared/ package) ──

type MappedEvent = shared.MappedEvent
type MapperRegistry = shared.MapperRegistry
type EventMapper = shared.EventMapper

var ErrMalformedPayload = shared.ErrMalformedPayload
var ErrUnsupportedEventType = shared.ErrUnsupportedEventType

func NewMapperRegistry() *MapperRegistry {
	return shared.NewMapperRegistry()
}

func ParseTimestamp(s string) (time.Time, error) {
	return shared.ParseTimestamp(s)
}

// ── Input type aliases (zero-copy pass-through to grouped packages) ──

type IngestEmailEventFactInput = ingestion.IngestFactCommand
type IngestOperationsEventInput = ingestion.IngestOpsCommand
type GetDashboardOverviewInput = dashboard.DashboardQuery
type GetCampaignAnalyticsInput = campaign.CampaignAnalyticsQuery
type GetCampaignFunnelInput = campaign.CampaignFunnelQuery
type GetCampaignTimeSeriesInput = campaign.CampaignTimeSeriesQuery
type GetCampaignBreakdownInput = campaign.CampaignBreakdownQuery
type GetCampaignEventsInput = campaign.CampaignEventsQuery
type GetDeliverabilityInput = deliverability.DeliverabilityQuery
type GetDeliverabilityTimeSeriesInput = deliverability.TimeSeriesQuery
type GetDeliverabilityBreakdownInput = deliverability.BreakdownQuery
type GetDeliverabilityLatencyInput = deliverability.LatencyQuery
type GetDeliverabilityIncidentsInput = deliverability.IncidentsQuery
type SearchEventsInput = forensics.SearchQuery
type GetMessageTimelineInput = forensics.MessageTimelineQuery
type GetProviderEventTraceInput = forensics.ProviderEventTraceQuery
type GetCampaignIncidentTimelineInput = forensics.IncidentTimelineQuery
type GetOutboxLagInput = operations.OutboxLagQuery
type GetConsumerFailuresInput = operations.ConsumerFailuresQuery
type GetDLQVolumeInput = operations.DLQVolumeQuery
type GetWebhookDeliveryTimeSeriesInput = operations.WebhookDeliveryTimeSeriesQuery
type GetWebhookReliabilityInput = operations.WebhookReliabilityQuery
type GetUsageTimeSeriesInput = usage.UsageTimeSeriesQuery
type GetUsageFeaturesInput = usage.UsageFeaturesQuery
type GetRiskSignalsInput = usage.RiskSignalsQuery
type GetSendVolumeForecastInput = usage.SendVolumeForecastQuery
type GetAnomaliesInput = anomaly.AnomaliesQuery
