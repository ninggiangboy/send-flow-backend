package campaign

const (
	Name    = "campaign"
	Purpose = "Manage email campaigns, scheduling, recipient snapshots, and lifecycle."
)

var (
	OwnedData = []string{"campaigns", "campaign_message_candidates"}
	Commands  = []string{"create_campaign", "update_campaign", "schedule_campaign", "cancel_campaign", "pause_campaign", "resume_campaign"}
	Queries   = []string{"list_campaigns", "get_campaign", "list_campaign_candidates"}
	Events    = []string{"campaign.scheduled.v1", "campaign.cancelled.v1", "campaign.paused.v1", "campaign.resumed.v1"}
)
