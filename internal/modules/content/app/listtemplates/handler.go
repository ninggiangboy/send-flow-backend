package listtemplates

import (
	"context"
	"errors"
	"log/slog"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/content/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/content/ports"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/constants"
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
		log:           opts.Logger.With("usecase", "list_templates"),
	}
}

type Query struct {
	WorkspaceID string
	Status      string
	Q           string
	Cursor      string
	Limit       int
	UserID      string
}

type Result struct {
	Templates  []domain.Template
	NextCursor string
}

func (h *Handler) Execute(ctx context.Context, q Query) (*Result, error) {
	if err := h.accessChecker.RequirePermission(ctx, q.WorkspaceID, q.UserID, "template.read"); err != nil {
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, err
		}
		return nil, err
	}

	if q.Limit <= 0 || q.Limit > 100 {
		q.Limit = constants.DefaultPageSize
	}

	templates, nextCursor, err := h.templatesRead.ListTemplates(ctx, q.WorkspaceID, q.Status, q.Q, q.Cursor, q.Limit)
	if err != nil {
		h.log.Error("failed to list templates", "error", err)
		return nil, err
	}

	return &Result{Templates: templates, NextCursor: nextCursor}, nil
}
