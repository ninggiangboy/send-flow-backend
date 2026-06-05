package content

const Name = "content"

const Purpose = "Own templates, template versions, render validation, and rendered snapshots."

var OwnedData = []string{
	"templates",
	"template_versions",
	"rendered_template_snapshots",
}

var Commands = []string{
	"CreateTemplate",
	"UpdateTemplateDraft",
	"PublishTemplateVersion",
	"ArchiveTemplate",
	"RenderTemplatePreview",
}

var Queries = []string{
	"GetTemplate",
	"GetPublishedTemplateVersion",
	"ValidateTemplateRenderable",
	"RenderForMessage",
}

var Events = []string{
	"content.template.created.v1",
	"content.template.published.v1",
	"content.template.render_failed.v1",
}
