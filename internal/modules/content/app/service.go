package app

import (
	"context"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/content/app/render"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/content/app/template"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/content/domain"
	contentredis "github.com/ninggiangboy/send-flow/backend/internal/modules/content/infrastructure/redis"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/content/ports"
)

type Options struct {
	TemplatesRead  ports.TemplateReadRepository
	TemplatesWrite ports.TemplateWriteRepository
	AccessChecker  ports.WorkspaceAccessChecker
	IDGen          func() (string, error)
	Logger         *slog.Logger
	RedisCache     *contentredis.Cache
}

type Service struct {
	createTemplateH              *template.CreateHandler
	listTemplatesH               *template.ListHandler
	getTemplateH                 *template.GetHandler
	updateTemplateH              *template.UpdateHandler
	publishTemplateH             *template.PublishHandler
	listTemplateVersionsH        *template.ListVersionsHandler
	previewTemplateH             *render.PreviewHandler
	renderH                      *render.Handler
	renderVersionH               *render.VersionHandler
	getPublishedTemplateVersionH *template.GetPublishedHandler
	validateTemplateRenderableH  *render.ValidateHandler
}

func NewService(opts Options) *Service {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.IDGen == nil {
		opts.IDGen = func() (string, error) { return "", nil }
	}

	return &Service{
		createTemplateH: template.NewCreateHandler(struct {
			TemplatesWrite ports.TemplateWriteRepository
			AccessChecker  ports.WorkspaceAccessChecker
			IDGen          func() (string, error)
			Logger         *slog.Logger
		}{
			TemplatesWrite: opts.TemplatesWrite,
			AccessChecker:  opts.AccessChecker,
			IDGen:          opts.IDGen,
			Logger:         opts.Logger,
		}),
		listTemplatesH: template.NewListHandler(struct {
			TemplatesRead ports.TemplateReadRepository
			AccessChecker ports.WorkspaceAccessChecker
			Logger        *slog.Logger
		}{
			TemplatesRead: opts.TemplatesRead,
			AccessChecker: opts.AccessChecker,
			Logger:        opts.Logger,
		}),
		getTemplateH: template.NewGetHandler(struct {
			TemplatesRead ports.TemplateReadRepository
			AccessChecker ports.WorkspaceAccessChecker
			Logger        *slog.Logger
		}{
			TemplatesRead: opts.TemplatesRead,
			AccessChecker: opts.AccessChecker,
			Logger:        opts.Logger,
		}),
		updateTemplateH: template.NewUpdateHandler(struct {
			TemplatesWrite ports.TemplateWriteRepository
			AccessChecker  ports.WorkspaceAccessChecker
			Logger         *slog.Logger
		}{
			TemplatesWrite: opts.TemplatesWrite,
			AccessChecker:  opts.AccessChecker,
			Logger:         opts.Logger,
		}),
		publishTemplateH: template.NewPublishHandler(struct {
			TemplatesWrite ports.TemplateWriteRepository
			AccessChecker  ports.WorkspaceAccessChecker
			IDGen          func() (string, error)
			Logger         *slog.Logger
		}{
			TemplatesWrite: opts.TemplatesWrite,
			AccessChecker:  opts.AccessChecker,
			IDGen:          opts.IDGen,
			Logger:         opts.Logger,
		}),
		listTemplateVersionsH: template.NewListVersionsHandler(struct {
			TemplatesRead ports.TemplateReadRepository
			AccessChecker ports.WorkspaceAccessChecker
			Logger        *slog.Logger
		}{
			TemplatesRead: opts.TemplatesRead,
			AccessChecker: opts.AccessChecker,
			Logger:        opts.Logger,
		}),
		previewTemplateH: render.NewPreviewHandler(struct {
			TemplatesRead ports.TemplateReadRepository
			AccessChecker ports.WorkspaceAccessChecker
			Logger        *slog.Logger
			Cache         *contentredis.Cache
		}{
			TemplatesRead: opts.TemplatesRead,
			AccessChecker: opts.AccessChecker,
			Logger:        opts.Logger,
			Cache:         opts.RedisCache,
		}),
		renderH: render.NewHandler(struct {
			TemplatesWrite ports.TemplateWriteRepository
			AccessChecker  ports.WorkspaceAccessChecker
			IDGen          func() (string, error)
			Logger         *slog.Logger
		}{
			TemplatesWrite: opts.TemplatesWrite,
			AccessChecker:  opts.AccessChecker,
			IDGen:          opts.IDGen,
			Logger:         opts.Logger,
		}),
		renderVersionH: render.NewVersionHandler(struct {
			TemplatesRead ports.TemplateReadRepository
			Logger        *slog.Logger
		}{
			TemplatesRead: opts.TemplatesRead,
			Logger:        opts.Logger,
		}),
		getPublishedTemplateVersionH: template.NewGetPublishedHandler(struct {
			TemplatesRead ports.TemplateReadRepository
			Logger        *slog.Logger
		}{
			TemplatesRead: opts.TemplatesRead,
			Logger:        opts.Logger,
		}),
		validateTemplateRenderableH: render.NewValidateHandler(struct {
			TemplatesRead ports.TemplateReadRepository
			Logger        *slog.Logger
		}{
			TemplatesRead: opts.TemplatesRead,
			Logger:        opts.Logger,
		}),
	}
}

func (s *Service) CreateTemplate(ctx context.Context, input CreateTemplateInput) (*TemplateResult, error) {
	return s.createTemplateH.Execute(ctx, input)
}

func (s *Service) ListTemplates(ctx context.Context, workspaceID, status, q, cursor string, limit int, userID string) (*TemplateListResult, error) {
	return s.listTemplatesH.Execute(ctx, template.ListQuery{
		WorkspaceID: workspaceID,
		Status:      status,
		Q:           q,
		Cursor:      cursor,
		Limit:       limit,
		UserID:      userID,
	})
}

func (s *Service) GetTemplate(ctx context.Context, workspaceID, templateID, userID string) (*TemplateResult, error) {
	tmpl, err := s.getTemplateH.Execute(ctx, template.GetQuery{
		WorkspaceID: workspaceID,
		TemplateID:  templateID,
		UserID:      userID,
	})
	if err != nil {
		return nil, err
	}
	return &TemplateResult{Template: *tmpl}, nil
}

func (s *Service) UpdateTemplate(ctx context.Context, input UpdateTemplateInput) (*TemplateResult, error) {
	result, err := s.updateTemplateH.Execute(ctx, input)
	if err != nil {
		return nil, err
	}
	return &TemplateResult{Template: result.Template}, nil
}

func (s *Service) PublishTemplate(ctx context.Context, workspaceID, templateID, userID string, now time.Time) (*VersionResult, error) {
	return s.publishTemplateH.Execute(ctx, template.PublishInput{
		WorkspaceID: workspaceID,
		TemplateID:  templateID,
		UserID:      userID,
		Now:         now,
	})
}

func (s *Service) ListTemplateVersions(ctx context.Context, workspaceID, templateID, userID string, limit int, cursor string) (*VersionListResult, error) {
	return s.listTemplateVersionsH.Execute(ctx, template.ListVersionsQuery{
		WorkspaceID: workspaceID,
		TemplateID:  templateID,
		UserID:      userID,
		Limit:       limit,
		Cursor:      cursor,
	})
}

func (s *Service) PreviewTemplate(ctx context.Context, workspaceID, templateID, userID string, templateData map[string]any, now time.Time) (*RenderResult, error) {
	return s.previewTemplateH.Execute(ctx, render.PreviewQuery{
		WorkspaceID:  workspaceID,
		TemplateID:   templateID,
		UserID:       userID,
		TemplateData: templateData,
	})
}

func (s *Service) Render(ctx context.Context, workspaceID, userID, templateID, subject, sourceHTML, sourceText string, templateData map[string]any, now time.Time) (*RenderResult, error) {
	return s.renderH.Execute(ctx, render.Query{
		WorkspaceID:  workspaceID,
		UserID:       userID,
		TemplateID:   templateID,
		Subject:      subject,
		SourceHTML:   sourceHTML,
		SourceText:   sourceText,
		TemplateData: templateData,
		Now:          now,
	})
}

func (s *Service) RenderVersion(ctx context.Context, workspaceID, templateVersionID string, data map[string]any) (*RenderResult, error) {
	rendered, err := s.renderVersionH.Execute(ctx, render.RenderVersionQuery{
		WorkspaceID:       workspaceID,
		TemplateVersionID: templateVersionID,
		Data:              data,
	})
	if err != nil {
		return nil, err
	}
	return &RenderResult{Result: *rendered}, nil
}

func (s *Service) GetPublishedTemplateVersion(ctx context.Context, workspaceID, templateID string) (*domain.TemplateVersion, error) {
	return s.getPublishedTemplateVersionH.Execute(ctx, template.GetPublishedQuery{
		WorkspaceID: workspaceID,
		TemplateID:  templateID,
	})
}

func (s *Service) ValidateTemplateRenderable(ctx context.Context, workspaceID, templateID string) error {
	return s.validateTemplateRenderableH.Execute(ctx, workspaceID, templateID)
}
