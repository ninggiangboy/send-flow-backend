package contracts

const (
	EventTemplateCreatedV1      = "content.template.created.v1"
	EventTemplatePublishedV1    = "content.template.published.v1"
	EventTemplateRenderFailedV1 = "content.template.render_failed.v1"
)

type TemplateCreatedPayload struct {
	TemplateID      string `json:"template_id"`
	WorkspaceID     string `json:"workspace_id"`
	Name            string `json:"name"`
	CreatedByUserID string `json:"created_by_user_id"`
	CreatedAt       string `json:"created_at"`
}

type TemplatePublishedPayload struct {
	TemplateID  string `json:"template_id"`
	WorkspaceID string `json:"workspace_id"`
	VersionID   string `json:"version_id"`
	PublishedAt string `json:"published_at"`
}

type TemplateRenderFailedPayload struct {
	TemplateID  string `json:"template_id"`
	WorkspaceID string `json:"workspace_id"`
	VersionID   string `json:"version_id"`
	Error       string `json:"error"`
	OccurredAt  string `json:"occurred_at"`
}
