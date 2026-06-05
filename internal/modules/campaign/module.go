package campaign

const Name = "campaign"

const Purpose = "Own campaign planning, scheduling, lifecycle state, and message candidates."

var OwnedData = []string{
	"campaigns",
	"campaign_message_candidates",
}

var Commands = []string{
	"CreateCampaign",
	"UpdateCampaignDraft",
	"ScheduleCampaign",
	"CancelCampaign",
	"PauseCampaign",
	"ResumeCampaign",
}

var Queries = []string{
	"GetCampaign",
	"ListCampaigns",
	"GetCampaignStatus",
	"ListCampaignCandidates",
}

var Events = []string{
	"campaign.created.v1",
	"campaign.scheduled.v1",
	"campaign.cancelled.v1",
	"campaign.paused.v1",
	"campaign.resumed.v1",
	"campaign.message_candidate.created.v1",
}
