package audit

const Name = "audit"

const Purpose = "Record append-only sensitive actions for review, compliance, and investigation."

var OwnedData = []string{
	"audit_entries",
}

var Commands = []string{
	"RecordAuditEntry",
}

var Queries = []string{
	"SearchAuditEntries",
	"GetAuditEntry",
}

var Events = []string{
	"audit.entry.recorded.v1",
}
