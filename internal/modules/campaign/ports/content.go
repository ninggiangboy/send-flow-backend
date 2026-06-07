package ports

import "context"

type TemplateVersion struct {
	ID            string
	TemplateID    string
	VersionNumber int
	Subject       string
	SourceHTML    string
	SourceText    string
	PublishedAt   string
	CreatedAt     string
}

type ContentService interface {
	GetPublishedTemplateVersion(ctx context.Context, workspaceID, templateID string) (*TemplateVersion, error)
	ValidateTemplateRenderable(ctx context.Context, workspaceID, templateID string) error
}
