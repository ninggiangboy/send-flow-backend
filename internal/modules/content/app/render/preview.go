package render

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/content/app/shared"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/content/domain"
	contentredis "github.com/ninggiangboy/send-flow/backend/internal/modules/content/infrastructure/redis"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/content/ports"
)

type PreviewQuery struct {
	WorkspaceID  string
	TemplateID   string
	UserID       string
	TemplateData map[string]any
}

type PreviewHandler struct {
	templatesRead ports.TemplateReadRepository
	accessChecker ports.WorkspaceAccessChecker
	log           *slog.Logger
	cache         *contentredis.Cache
}

func NewPreviewHandler(opts struct {
	TemplatesRead ports.TemplateReadRepository
	AccessChecker ports.WorkspaceAccessChecker
	Logger        *slog.Logger
	Cache         *contentredis.Cache
}) *PreviewHandler {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	return &PreviewHandler{
		templatesRead: opts.TemplatesRead,
		accessChecker: opts.AccessChecker,
		log:           opts.Logger.With("usecase", "preview_template"),
		cache:         opts.Cache,
	}
}

func (h *PreviewHandler) Execute(ctx context.Context, q PreviewQuery) (*Result, error) {
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
			r, renderErr := shared.RenderTemplateSource(tmpl.Subject, tmpl.SourceHTML, tmpl.SourceText, q.TemplateData)
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
		return &Result{Result: domain.RenderResult{
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

	r, err := shared.RenderTemplateSource(tmpl.Subject, tmpl.SourceHTML, tmpl.SourceText, q.TemplateData)
	if err != nil {
		return nil, err
	}

	return &Result{Result: *r}, nil
}

func payloadHash(data map[string]any) string {
	if len(data) == 0 {
		return ""
	}
	b, _ := json.Marshal(data)
	return fmt.Sprintf("%x", sha256.Sum256(b))
}
