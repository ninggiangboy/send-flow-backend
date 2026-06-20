package template

import (
	"context"
	"errors"
	"log/slog"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/content/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/content/ports"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/constants"
)

// --- GetTemplate ---

type GetQuery struct {
	WorkspaceID string
	TemplateID  string
	UserID      string
}

type GetHandler struct {
	templatesRead ports.TemplateReadRepository
	accessChecker ports.WorkspaceAccessChecker
	log           *slog.Logger
}

func NewGetHandler(opts struct {
	TemplatesRead ports.TemplateReadRepository
	AccessChecker ports.WorkspaceAccessChecker
	Logger        *slog.Logger
}) *GetHandler {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	return &GetHandler{
		templatesRead: opts.TemplatesRead,
		accessChecker: opts.AccessChecker,
		log:           opts.Logger.With("usecase", "get_template"),
	}
}

func (h *GetHandler) Execute(ctx context.Context, q GetQuery) (*domain.Template, error) {
	if err := h.accessChecker.RequirePermission(ctx, q.WorkspaceID, q.UserID, "template.read"); err != nil {
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, err
		}
		return nil, err
	}

	tmpl, err := h.templatesRead.FindTemplateByID(ctx, q.WorkspaceID, q.TemplateID)
	if err != nil {
		if errors.Is(err, domain.ErrTemplateNotFound) {
			return nil, err
		}
		h.log.Error("failed to find template", "error", err)
		return nil, err
	}

	return tmpl, nil
}

// --- ListTemplates ---

type ListQuery struct {
	WorkspaceID string
	Status      string
	Q           string
	Cursor      string
	Limit       int
	UserID      string
}

type ListResult struct {
	Templates  []domain.Template
	NextCursor string
}

type ListHandler struct {
	templatesRead ports.TemplateReadRepository
	accessChecker ports.WorkspaceAccessChecker
	log           *slog.Logger
}

func NewListHandler(opts struct {
	TemplatesRead ports.TemplateReadRepository
	AccessChecker ports.WorkspaceAccessChecker
	Logger        *slog.Logger
}) *ListHandler {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	return &ListHandler{
		templatesRead: opts.TemplatesRead,
		accessChecker: opts.AccessChecker,
		log:           opts.Logger.With("usecase", "list_templates"),
	}
}

func (h *ListHandler) Execute(ctx context.Context, q ListQuery) (*ListResult, error) {
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

	return &ListResult{Templates: templates, NextCursor: nextCursor}, nil
}

// --- ListTemplateVersions ---

type ListVersionsQuery struct {
	WorkspaceID string
	TemplateID  string
	UserID      string
	Limit       int
	Cursor      string
}

type ListVersionsResult struct {
	Versions   []domain.TemplateVersion
	NextCursor string
}

type ListVersionsHandler struct {
	templatesRead ports.TemplateReadRepository
	accessChecker ports.WorkspaceAccessChecker
	log           *slog.Logger
}

func NewListVersionsHandler(opts struct {
	TemplatesRead ports.TemplateReadRepository
	AccessChecker ports.WorkspaceAccessChecker
	Logger        *slog.Logger
}) *ListVersionsHandler {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	return &ListVersionsHandler{
		templatesRead: opts.TemplatesRead,
		accessChecker: opts.AccessChecker,
		log:           opts.Logger.With("usecase", "list_template_versions"),
	}
}

func (h *ListVersionsHandler) Execute(ctx context.Context, q ListVersionsQuery) (*ListVersionsResult, error) {
	if err := h.accessChecker.RequirePermission(ctx, q.WorkspaceID, q.UserID, "template.read"); err != nil {
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, err
		}
		return nil, err
	}

	if q.Limit <= 0 || q.Limit > 100 {
		q.Limit = constants.DefaultPageSize
	}

	versions, nextCursor, err := h.templatesRead.ListTemplateVersions(ctx, q.WorkspaceID, q.TemplateID, q.Cursor, q.Limit)
	if err != nil {
		h.log.Error("failed to list template versions", "error", err)
		return nil, err
	}

	return &ListVersionsResult{Versions: versions, NextCursor: nextCursor}, nil
}

// --- GetPublishedTemplateVersion ---

type GetPublishedQuery struct {
	WorkspaceID string
	TemplateID  string
}

type GetPublishedHandler struct {
	templatesRead ports.TemplateReadRepository
	log           *slog.Logger
}

func NewGetPublishedHandler(opts struct {
	TemplatesRead ports.TemplateReadRepository
	Logger        *slog.Logger
}) *GetPublishedHandler {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	return &GetPublishedHandler{
		templatesRead: opts.TemplatesRead,
		log:           opts.Logger.With("usecase", "get_published_template_version"),
	}
}

func (h *GetPublishedHandler) Execute(ctx context.Context, q GetPublishedQuery) (*domain.TemplateVersion, error) {
	version, err := h.templatesRead.FindCurrentVersion(ctx, q.WorkspaceID, q.TemplateID)
	if err != nil {
		if errors.Is(err, domain.ErrTemplateVersionNotFound) {
			return nil, err
		}
		h.log.Error("failed to find current version", "error", err)
		return nil, err
	}
	return version, nil
}
