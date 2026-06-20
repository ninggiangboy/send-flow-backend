package render

import (
	"context"
	"errors"
	"log/slog"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/content/app/shared"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/content/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/content/ports"
)

type RenderVersionQuery struct {
	WorkspaceID       string
	TemplateVersionID string
	Data              map[string]any
}

type VersionHandler struct {
	templatesRead ports.TemplateReadRepository
	log           *slog.Logger
}

func NewVersionHandler(opts struct {
	TemplatesRead ports.TemplateReadRepository
	Logger        *slog.Logger
}) *VersionHandler {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	return &VersionHandler{
		templatesRead: opts.TemplatesRead,
		log:           opts.Logger.With("usecase", "render_version"),
	}
}

func (h *VersionHandler) Execute(ctx context.Context, q RenderVersionQuery) (*domain.RenderResult, error) {
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

	rendered, err := shared.RenderTemplateSource(version.Subject, version.SourceHTML, version.SourceText, q.Data)
	if err != nil {
		return nil, err
	}

	return rendered, nil
}
