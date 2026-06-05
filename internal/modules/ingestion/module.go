package ingestion

const Name = "ingestion"

const Purpose = "Receive, verify, persist, dedupe, and normalize provider webhook events."

var OwnedData = []string{
	"provider_webhook_events",
	"normalized_provider_events",
}

var Commands = []string{
	"IngestProviderWebhook",
	"NormalizeProviderEvent",
	"MarkProviderEventProcessed",
}

var Queries = []string{
	"GetRawProviderEvent",
	"GetNormalizedProviderEvent",
}

var Events = []string{
	"ingestion.provider_webhook.received.v1",
	"ingestion.provider_event.normalized.v1",
}
