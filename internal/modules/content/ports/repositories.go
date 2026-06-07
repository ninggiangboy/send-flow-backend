package ports

import (
	"context"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/content/domain"
)

type TemplateListQuery struct {
	WorkspaceID string
	Status      string
	Q           string
	Cursor      string
	Limit       int
}

type VersionListQuery struct {
	WorkspaceID string
	TemplateID  string
	Limit       int
	Cursor      string
}

type TemplateReadRepository interface {
	FindTemplateByID(ctx context.Context, workspaceID, templateID string) (*domain.Template, error)
	ListTemplates(ctx context.Context, query TemplateListQuery) ([]domain.Template, string, error)
	ListTemplateVersions(ctx context.Context, query VersionListQuery) ([]domain.TemplateVersion, string, error)
	FindTemplateVersionByID(ctx context.Context, workspaceID, versionID string) (*domain.TemplateVersion, error)
	FindCurrentVersion(ctx context.Context, workspaceID, templateID string) (*domain.TemplateVersion, error)
}

type TemplateWriteRepository interface {
	CreateTemplate(ctx context.Context, template domain.Template) error
	UpdateTemplate(ctx context.Context, template domain.Template) error
	PublishTemplateVersion(ctx context.Context, template domain.Template, version domain.TemplateVersion) error
	CreateRenderSnapshot(ctx context.Context, snapshot domain.RenderedTemplateSnapshot) error
}
