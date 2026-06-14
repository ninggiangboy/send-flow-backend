package gettemplate

import (
	"context"
	"errors"
	"log/slog"

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
		log:           opts.Logger.With("usecase", "get_template"),
	}
}

type Result struct {
	Template domain.Template
}

func (h *Handler) Execute(ctx context.Context, workspaceID, templateID, userID string) (*Result, error) {
	if err := h.accessChecker.RequirePermission(ctx, workspaceID, userID, "template.read"); err != nil {
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, err
		}
		return nil, err
	}

	tmpl, err := h.templatesRead.FindTemplateByID(ctx, workspaceID, templateID)
	if err != nil {
		if errors.Is(err, domain.ErrTemplateNotFound) {
			return nil, err
		}
		h.log.Error("failed to find template", "error", err)
		return nil, err
	}

	return &Result{Template: *tmpl}, nil
}
