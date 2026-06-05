package tracking

const Name = "tracking"

const Purpose = "Own tracking links, open and click endpoints, and recipient behavior events."

var OwnedData = []string{
	"tracking_links",
	"tracking_events",
}

var Commands = []string{
	"CreateTrackingLink",
	"RecordEmailOpened",
	"RecordLinkClicked",
}

var Queries = []string{
	"ResolveTrackingLink",
	"GetTrackingEvent",
}

var Events = []string{
	"tracking.email_opened.v1",
	"tracking.link_clicked.v1",
}
