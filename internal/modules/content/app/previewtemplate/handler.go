package previewtemplate

import (
	"context"
	"errors"
	"log/slog"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/content/app/render"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/content/app/usecase"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/content/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/content/ports"
)

type Options struct {
	TemplatesRead ports.TemplateReadRepository
	AccessChecker ports.WorkspaceAccessChecker
	Logger        *slog.Logger
}

type Handler struct {
	templatesRead ports.TemplateReadRepository
	accessChecker ports.WorkspaceAccessChecker
	log           *slog.Logger
}

func New(opts Options) *Handler {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	return &Handler{
		templatesRead: opts.TemplatesRead,
		accessChecker: opts.AccessChecker,
		log:           opts.Logger.With("usecase", "preview_template"),
	}
}

type Query struct {
	WorkspaceID  string
	TemplateID   string
	UserID       string
	TemplateData map[string]any
}

func (h *Handler) Execute(ctx context.Context, q Query) (*render.Result, error) {
	if err := h.accessChecker.RequirePermission(ctx, q.WorkspaceID, q.UserID, "template.render"); err != nil {
		if errors.Is(err, domain.ErrRenderDenied) {
			return nil, err
		}
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, domain.ErrRenderDenied
		}
		return nil, err
	}

	tmpl, err := h.templatesRead.FindTemplateByID(ctx, q.WorkspaceID, q.TemplateID)
	if err != nil {
		if errors.Is(err, domain.ErrTemplateNotFound) {
			return nil, err
		}
		h.log.Error("failed to find template for preview", "error", err)
		return nil, err
	}

	r, err := usecase.RenderTemplateSource(tmpl.Subject, tmpl.SourceHTML, tmpl.SourceText, q.TemplateData)
	if err != nil {
		return nil, err
	}

	return &render.Result{Result: *r}, nil
}
