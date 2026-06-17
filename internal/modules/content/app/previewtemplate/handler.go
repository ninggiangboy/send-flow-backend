package previewtemplate

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/content/app/render"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/content/app/usecase"
	contentredis "github.com/ninggiangboy/send-flow/backend/internal/modules/content/infrastructure/redis"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/content/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/content/ports"
)

type Options struct {
	TemplatesRead ports.TemplateReadRepository
	AccessChecker ports.WorkspaceAccessChecker
	Logger        *slog.Logger
	Cache         *contentredis.Cache
}

type Handler struct {
	templatesRead ports.TemplateReadRepository
	accessChecker ports.WorkspaceAccessChecker
	log           *slog.Logger
	cache         *contentredis.Cache
}

func New(opts Options) *Handler {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	return &Handler{
		templatesRead: opts.TemplatesRead,
		accessChecker: opts.AccessChecker,
		log:           opts.Logger.With("usecase", "preview_template"),
		cache:         opts.Cache,
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

	if h.cache != nil {
		hash := payloadHash(q.TemplateData)
		cached, err := h.cache.GetOrLoadPreview(ctx, q.WorkspaceID, q.TemplateID, hash, func() (*contentredis.PreviewResult, error) {
			tmpl, loadErr := h.templatesRead.FindTemplateByID(ctx, q.WorkspaceID, q.TemplateID)
			if loadErr != nil {
				return nil, loadErr
			}
			r, renderErr := usecase.RenderTemplateSource(tmpl.Subject, tmpl.SourceHTML, tmpl.SourceText, q.TemplateData)
			if renderErr != nil {
				return nil, renderErr
			}
			return &contentredis.PreviewResult{
				Subject:  r.Subject,
				HTML:     r.HTML,
				Text:     r.Text,
				Warnings: r.Warnings,
			}, nil
		})
		if err != nil {
			return nil, err
		}
		return &render.Result{Result: domain.RenderResult{
			Subject:  cached.Subject,
			HTML:     cached.HTML,
			Text:     cached.Text,
			Warnings: cached.Warnings,
		}}, nil
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

func payloadHash(data map[string]any) string {
	if len(data) == 0 {
		return ""
	}
	b, _ := json.Marshal(data)
	return fmt.Sprintf("%x", sha256.Sum256(b))
}
