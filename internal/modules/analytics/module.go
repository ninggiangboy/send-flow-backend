package analytics

const Name = "analytics"

const Purpose = "Own email event facts in ClickHouse and provide dashboard, campaign, deliverability, forensic, operations, and usage analytics."

var OwnedData = []string{
	"email_events (ClickHouse)",
}

var Commands = []string{
	"IngestEmailEventFact",
}

var Queries = []string{
	"GetDashboardOverview",
	"GetCampaignAnalytics",
	"GetCampaignFunnel",
	"GetDeliverability",
	"SearchEvents",
	"GetMessageTimeline",
}

var Events = []string{}
