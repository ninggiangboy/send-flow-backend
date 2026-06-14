package renderversion

import (
	"context"
	"errors"
	"log/slog"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/content/app/usecase"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/content/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/content/ports"
)

type Options struct {
	TemplatesRead ports.TemplateReadRepository
	Logger        *slog.Logger
}

type Handler struct {
	templatesRead ports.TemplateReadRepository
	log           *slog.Logger
}

func New(opts Options) *Handler {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	return &Handler{
		templatesRead: opts.TemplatesRead,
		log:           opts.Logger.With("usecase", "render_version"),
	}
}

type Query struct {
	WorkspaceID       string
	TemplateVersionID string
	Data              map[string]any
}

type Result struct {
	Result domain.RenderResult
}

func (h *Handler) Execute(ctx context.Context, q Query) (*Result, error) {
	if q.Data == nil {
		q.Data = map[string]any{}
	}

	version, err := h.templatesRead.FindTemplateVersionByID(ctx, q.WorkspaceID, q.TemplateVersionID)
	if err != nil {
		if errors.Is(err, domain.ErrTemplateVersionNotFound) {
			return nil, err
		}
		h.log.Error("failed to find template version for render", "error", err)
		return nil, err
	}

	rendered, err := usecase.RenderTemplateSource(version.Subject, version.SourceHTML, version.SourceText, q.Data)
	if err != nil {
		return nil, err
	}

	return &Result{Result: *rendered}, nil
}
