package notification

const Name = "notification"

const Purpose = "Send internal product notifications such as welcome emails, invitations, and system alerts."

var OwnedData = []string{
	"notification_messages",
	"notification_attempts",
}

var Commands = []string{
	"SendWelcomeEmail",
	"SendWorkspaceInvitationEmail",
	"SendSystemAlert",
}

var Queries = []string{
	"GetNotificationStatus",
}

var Events = []string{
	"notification.message.queued.v1",
	"notification.message.sent.v1",
	"notification.message.failed.v1",
}
