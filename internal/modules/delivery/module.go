package delivery

const Name = "delivery"

const Purpose = "Own message lifecycle, queueing, provider routing, attempts, retry, and delivery state."

var OwnedData = []string{
	"transactional_send_requests",
	"messages",
	"delivery_attempts",
	"retry_states",
}

var Commands = []string{
	"AcceptTransactionalSend",
	"QueueCampaignMessages",
	"QueueMessage",
	"StartDeliveryAttempt",
	"MarkMessageAccepted",
	"MarkMessageDelivered",
	"MarkMessageBounced",
	"MarkMessageComplained",
	"ScheduleRetry",
	"MoveMessageToDLQ",
}

var Queries = []string{
	"GetMessage",
	"GetMessageByProviderMessageID",
	"ListMessageLogs",
	"GetQueueState",
	"GetRetryState",
}

var Events = []string{
	"delivery.transactional_send.accepted.v1",
	"delivery.message.queued.v1",
	"delivery.message.accepted.v1",
	"delivery.message.delivered.v1",
	"delivery.message.bounced.v1",
	"delivery.message.complained.v1",
	"delivery.message.retry_scheduled.v1",
}
