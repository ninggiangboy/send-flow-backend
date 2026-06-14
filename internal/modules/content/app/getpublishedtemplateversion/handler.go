package getpublishedtemplateversion

import (
	"context"
	"errors"
	"log/slog"

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
		log:           opts.Logger.With("usecase", "get_published_template_version"),
	}
}

type Result struct {
	Version *domain.TemplateVersion
}

func (h *Handler) Execute(ctx context.Context, workspaceID, templateID string) (*Result, error) {
	version, err := h.templatesRead.FindCurrentVersion(ctx, workspaceID, templateID)
	if err != nil {
		if errors.Is(err, domain.ErrTemplateVersionNotFound) {
			return nil, err
		}
		h.log.Error("failed to find current version", "error", err)
		return nil, err
	}
	return &Result{Version: version}, nil
}
