package publishtemplate

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/content/app/usecase"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/content/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/content/ports"
)

type Options struct {
	TemplatesRead  ports.TemplateReadRepository
	TemplatesWrite ports.TemplateWriteRepository
	AccessChecker  ports.WorkspaceAccessChecker
	IDGen          func() (string, error)
	Logger         *slog.Logger
}

type Handler struct {
	templatesRead  ports.TemplateReadRepository
	templatesWrite ports.TemplateWriteRepository
	accessChecker  ports.WorkspaceAccessChecker
	idGen          func() (string, error)
	log            *slog.Logger
}

func New(opts Options) *Handler {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	return &Handler{
		templatesRead:  opts.TemplatesRead,
		templatesWrite: opts.TemplatesWrite,
		accessChecker:  opts.AccessChecker,
		idGen:          opts.IDGen,
		log:            opts.Logger.With("usecase", "publish_template"),
	}
}

type Result struct {
	Version domain.TemplateVersion
}

func (h *Handler) Execute(ctx context.Context, workspaceID, templateID, userID string, now time.Time) (*Result, error) {
	if err := h.accessChecker.RequirePermission(ctx, workspaceID, userID, "template.write"); err != nil {
		if errors.Is(err, domain.ErrWriteDenied) {
			return nil, err
		}
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, domain.ErrWriteDenied
		}
		return nil, err
	}

	tmpl, err := h.templatesRead.FindTemplateByID(ctx, workspaceID, templateID)
	if err != nil {
		if errors.Is(err, domain.ErrTemplateNotFound) {
			return nil, err
		}
		h.log.Error("failed to find template for publish", "error", err)
		return nil, err
	}

	if tmpl.Status == domain.TemplateStatusArchived {
		return nil, domain.ErrPublishConflict
	}

	if err := usecase.ValidateTemplateSource(tmpl.Subject, tmpl.SourceHTML, tmpl.SourceText); err != nil {
		return nil, err
	}

	versions, _, err := h.templatesRead.ListTemplateVersions(ctx, workspaceID, templateID, "", 1)
	if err != nil {
		h.log.Error("failed to list versions for version number", "error", err)
		return nil, err
	}

	nextVersion := 1
	if len(versions) > 0 {
		nextVersion = versions[0].VersionNumber + 1
	}

	verID, err := h.idGen()
	if err != nil {
		h.log.Error("failed to generate id", "error", err)
		return nil, err
	}

	version := domain.TemplateVersion{
		ID:            verID,
		WorkspaceID:   workspaceID,
		TemplateID:    templateID,
		VersionNumber: nextVersion,
		Subject:       tmpl.Subject,
		SourceHTML:    tmpl.SourceHTML,
		SourceText:    tmpl.SourceText,
		Metadata:      tmpl.Metadata,
		PublishedAt:   now,
		CreatedAt:     now,
	}

	tmpl.Status = domain.TemplateStatusActive
	tmpl.CurrentVersionID = verID
	tmpl.UpdatedAt = now

	if err := h.templatesWrite.PublishTemplateVersion(ctx, *tmpl, version); err != nil {
		h.log.Error("failed to publish template version", "error", err)
		return nil, err
	}

	h.log.Info("template published", "template_id", templateID, "version_id", verID, "version_number", nextVersion)
	return &Result{Version: version}, nil
}
