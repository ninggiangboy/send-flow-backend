package render

import (
	"context"
	"errors"
	"log/slog"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/content/app/shared"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/content/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/content/ports"
)

type ValidateHandler struct {
	templatesRead ports.TemplateReadRepository
	log           *slog.Logger
}

func NewValidateHandler(opts struct {
	TemplatesRead ports.TemplateReadRepository
	Logger        *slog.Logger
}) *ValidateHandler {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	return &ValidateHandler{
		templatesRead: opts.TemplatesRead,
		log:           opts.Logger.With("usecase", "validate_template_renderable"),
	}
}

func (h *ValidateHandler) Execute(ctx context.Context, workspaceID, templateID string) error {
	tmpl, err := h.templatesRead.FindTemplateByID(ctx, workspaceID, templateID)
	if err != nil {
		if errors.Is(err, domain.ErrTemplateNotFound) {
			return err
		}
		h.log.Error("failed to find template", "error", err)
		return err
	}

	if err := shared.ValidateTemplateSource(tmpl.Subject, tmpl.SourceHTML, tmpl.SourceText); err != nil {
		return err
	}

	return nil
}
