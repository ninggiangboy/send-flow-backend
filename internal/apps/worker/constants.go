package worker

const (
	logFieldWorker           = "worker"
	logFieldConsumer         = "consumer"
	logFieldComponent        = "component"
	logFieldPollingGuard     = "polling_guard"
	logFieldElectedScheduler = "elected_scheduler"

	consumerDeliveryDueMessage     = "delivery.due_message_consumer"
	consumerDeliveryProviderEvents = "delivery_provider_events"
	consumerDeliveryQueueCampaign  = "delivery.queue_campaign_messages"
	consumerTrackingProviderEvents = "tracking_provider_events"
	consumerNotificationDue        = "notification.due_notification_consumer"
	consumerNotificationIdentity   = "notification.identity_events"
	consumerAnalyticsEvents        = "analytics_events"
	consumerWebhooksDue            = "webhooks.due_webhook_consumer"
	consumerWebhooksDeliver        = "webhooks.deliver_events"
)
