package analytics

const Name = "analytics"

const Purpose = "Own event facts, reporting projections, freshness metadata, and dashboard analytics."

var OwnedData = []string{
	"email_event_facts",
	"campaign_delivery_summaries",
	"recipient_domain_hourly_stats",
	"provider_delivery_stats",
	"dashboard_overviews",
}

var Commands = []string{
	"IngestEmailEventFact",
	"RebuildCampaignSummary",
	"BackfillAnalyticsWindow",
}

var Queries = []string{
	"GetDashboardOverview",
	"GetCampaignAnalytics",
	"GetProviderStats",
	"GetRecipientDomainStats",
}

var Events = []string{
	"analytics.projection.updated.v1",
	"analytics.backfill.completed.v1",
}
