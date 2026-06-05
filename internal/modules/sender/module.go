package sender

const Name = "sender"

const Purpose = "Own workspace sending identities, DNS readiness, and verified sender state."

var OwnedData = []string{
	"sender_domains",
	"domain_dns_record_statuses",
}

var Commands = []string{
	"CreateSenderDomain",
	"StartSenderDomainVerification",
	"RefreshSenderDomainDNSStatus",
	"MarkSenderDomainVerified",
	"DisableSenderDomain",
}

var Queries = []string{
	"GetSenderDomain",
	"GetSenderReadiness",
	"ListSenderDomains",
}

var Events = []string{
	"sender.domain.created.v1",
	"sender.domain.verified.v1",
	"sender.domain.disabled.v1",
}
