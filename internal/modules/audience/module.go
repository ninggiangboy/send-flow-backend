package audience

const Name = "audience"

const Purpose = "Own recipient identity, lists, segments, and audience import/export workflows."

var OwnedData = []string{
	"contacts",
	"audience_lists",
	"audience_list_memberships",
	"segments",
	"audience_import_jobs",
	"audience_export_jobs",
}

var Commands = []string{
	"CreateContact",
	"UpdateContact",
	"ArchiveContact",
	"CreateAudienceList",
	"AddContactToList",
	"CreateSegment",
	"StartAudienceImport",
	"StartAudienceExport",
}

var Queries = []string{
	"GetContact",
	"ListContacts",
	"ResolveAudienceSelection",
	"EstimateAudienceSize",
	"GetImportJobStatus",
}

var Events = []string{
	"audience.contact.created.v1",
	"audience.contact.updated.v1",
	"audience.segment.changed.v1",
	"audience.import.completed.v1",
	"audience.export.completed.v1",
}
