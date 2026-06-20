package app

import (
	"context"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/content/app/createtemplate"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/content/app/getpublishedtemplateversion"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/content/app/gettemplate"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/content/app/listtemplates"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/content/app/listtemplateversions"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/content/app/previewtemplate"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/content/app/publishtemplate"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/content/app/render"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/content/app/renderversion"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/content/app/updatetemplate"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/content/app/validatetemplaterenderable"
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
	createTemplateH              *createtemplate.Handler
	listTemplatesH               *listtemplates.Handler
	getTemplateH                 *gettemplate.Handler
	updateTemplateH              *updatetemplate.Handler
	publishTemplateH             *publishtemplate.Handler
	listTemplateVersionsH        *listtemplateversions.Handler
	previewTemplateH             *previewtemplate.Handler
	renderH                      *render.Handler
	renderVersionH               *renderversion.Handler
	getPublishedTemplateVersionH *getpublishedtemplateversion.Handler
	validateTemplateRenderableH  *validatetemplaterenderable.Handler
}

// Input types exposed to external callers.
type (
	CreateTemplateInput = createtemplate.Input
	UpdateTemplateInput = updatetemplate.Input
)

// Result types exposed to external callers.
type (
	TemplateResult     = createtemplate.Result
	TemplateListResult = listtemplates.Result
	VersionResult      = publishtemplate.Result
	VersionListResult  = listtemplateversions.Result
	RenderResult       = render.Result
)

func NewService(opts Options) *Service {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.IDGen == nil {
		opts.IDGen = func() (string, error) { return "", nil }
	}

	return &Service{
		createTemplateH: createtemplate.New(createtemplate.Options{
			TemplatesWrite: opts.TemplatesWrite,
			AccessChecker:  opts.AccessChecker,
			IDGen:          opts.IDGen,
			Logger:         opts.Logger,
		}),
		listTemplatesH: listtemplates.New(listtemplates.Options{
			TemplatesRead: opts.TemplatesRead,
			AccessChecker: opts.AccessChecker,
			Logger:        opts.Logger,
		}),
		getTemplateH: gettemplate.New(gettemplate.Options{
			TemplatesRead: opts.TemplatesRead,
			AccessChecker: opts.AccessChecker,
			Logger:        opts.Logger,
		}),
		updateTemplateH: updatetemplate.New(updatetemplate.Options{
			TemplatesWrite: opts.TemplatesWrite,
			AccessChecker:  opts.AccessChecker,
			Logger:         opts.Logger,
		}),
		publishTemplateH: publishtemplate.New(publishtemplate.Options{
			TemplatesWrite: opts.TemplatesWrite,
			AccessChecker:  opts.AccessChecker,
			IDGen:          opts.IDGen,
			Logger:         opts.Logger,
		}),
		listTemplateVersionsH: listtemplateversions.New(listtemplateversions.Options{
			TemplatesRead: opts.TemplatesRead,
			AccessChecker: opts.AccessChecker,
			Logger:        opts.Logger,
		}),
		previewTemplateH: previewtemplate.New(previewtemplate.Options{
			TemplatesRead: opts.TemplatesRead,
			AccessChecker: opts.AccessChecker,
			Logger:        opts.Logger,
			Cache:         opts.RedisCache,
		}),
		renderH: render.New(render.Options{
			TemplatesWrite: opts.TemplatesWrite,
			AccessChecker:  opts.AccessChecker,
			IDGen:          opts.IDGen,
			Logger:         opts.Logger,
		}),
		renderVersionH: renderversion.New(renderversion.Options{
			TemplatesRead: opts.TemplatesRead,
			Logger:        opts.Logger,
		}),
		getPublishedTemplateVersionH: getpublishedtemplateversion.New(getpublishedtemplateversion.Options{
			TemplatesRead: opts.TemplatesRead,
			Logger:        opts.Logger,
		}),
		validateTemplateRenderableH: validatetemplaterenderable.New(validatetemplaterenderable.Options{
			TemplatesRead: opts.TemplatesRead,
			Logger:        opts.Logger,
		}),
	}
}

func (s *Service) CreateTemplate(ctx context.Context, input CreateTemplateInput) (*TemplateResult, error) {
	return s.createTemplateH.Execute(ctx, input)
}

func (s *Service) ListTemplates(ctx context.Context, workspaceID, status, q, cursor string, limit int, userID string) (*TemplateListResult, error) {
	return s.listTemplatesH.Execute(ctx, listtemplates.Query{
		WorkspaceID: workspaceID,
		Status:      status,
		Q:           q,
		Cursor:      cursor,
		Limit:       limit,
		UserID:      userID,
	})
}

func (s *Service) GetTemplate(ctx context.Context, workspaceID, templateID, userID string) (*TemplateResult, error) {
	result, err := s.getTemplateH.Execute(ctx, workspaceID, templateID, userID)
	if err != nil {
		return nil, err
	}
	return &TemplateResult{Template: result.Template}, nil
}

func (s *Service) UpdateTemplate(ctx context.Context, input UpdateTemplateInput) (*TemplateResult, error) {
	result, err := s.updateTemplateH.Execute(ctx, input)
	if err != nil {
		return nil, err
	}
	return &TemplateResult{Template: result.Template}, nil
}

func (s *Service) PublishTemplate(ctx context.Context, workspaceID, templateID, userID string, now time.Time) (*VersionResult, error) {
	return s.publishTemplateH.Execute(ctx, workspaceID, templateID, userID, now)
}

func (s *Service) ListTemplateVersions(ctx context.Context, workspaceID, templateID, userID string, limit int, cursor string) (*VersionListResult, error) {
	return s.listTemplateVersionsH.Execute(ctx, listtemplateversions.Query{
		WorkspaceID: workspaceID,
		TemplateID:  templateID,
		UserID:      userID,
		Limit:       limit,
		Cursor:      cursor,
	})
}

func (s *Service) PreviewTemplate(ctx context.Context, workspaceID, templateID, userID string, templateData map[string]any, now time.Time) (*RenderResult, error) {
	return s.previewTemplateH.Execute(ctx, previewtemplate.Query{
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
	result, err := s.renderVersionH.Execute(ctx, renderversion.Query{
		WorkspaceID:       workspaceID,
		TemplateVersionID: templateVersionID,
		Data:              data,
	})
	if err != nil {
		return nil, err
	}
	return &RenderResult{Result: result.Result}, nil
}

func (s *Service) GetPublishedTemplateVersion(ctx context.Context, workspaceID, templateID string) (*domain.TemplateVersion, error) {
	result, err := s.getPublishedTemplateVersionH.Execute(ctx, workspaceID, templateID)
	if err != nil {
		return nil, err
	}
	return result.Version, nil
}

func (s *Service) ValidateTemplateRenderable(ctx context.Context, workspaceID, templateID string) error {
	return s.validateTemplateRenderableH.Execute(ctx, workspaceID, templateID)
}
