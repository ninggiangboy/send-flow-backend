package webhooks

const Name = "webhooks"

const Purpose = "Own outbound customer webhook configuration, delivery attempts, retry, and DLQ."

var OwnedData = []string{
	"customer_webhooks",
	"customer_webhook_deliveries",
}

var Commands = []string{
	"CreateWebhookConfig",
	"UpdateWebhookConfig",
	"DisableWebhookConfig",
	"DeliverCustomerWebhook",
	"RetryCustomerWebhookDelivery",
}

var Queries = []string{
	"ListWebhookConfigs",
	"GetWebhookDeliveryHistory",
	"GetSubscribedWebhookTargets",
}

var Events = []string{
	"webhooks.config.created.v1",
	"webhooks.delivery.succeeded.v1",
	"webhooks.delivery.failed.v1",
}
