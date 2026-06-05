package suppression

const Name = "suppression"

const Purpose = "Own recipient send-block decisions by scope and reason."

var OwnedData = []string{
	"suppression_entries",
}

var Commands = []string{
	"SuppressRecipient",
	"UnsuppressRecipient",
	"HandleUnsubscribe",
	"HandleComplaint",
	"HandleHardBounce",
}

var Queries = []string{
	"CheckSuppression",
	"ListSuppressionEntries",
	"GetSuppressionReason",
}

var Events = []string{
	"suppression.recipient_suppressed.v1",
	"suppression.recipient_unsuppressed.v1",
}
