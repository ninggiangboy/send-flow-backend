package template

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/content/app/shared"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/content/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/content/ports"
)

type PublishInput struct {
	WorkspaceID string
	TemplateID  string
	UserID      string
	Now         time.Time
}

type PublishResult struct {
	Version domain.TemplateVersion
}

type PublishHandler struct {
	templatesWrite ports.TemplateWriteRepository
	accessChecker  ports.WorkspaceAccessChecker
	idGen          func() (string, error)
	log            *slog.Logger
}

func NewPublishHandler(opts struct {
	TemplatesWrite ports.TemplateWriteRepository
	AccessChecker  ports.WorkspaceAccessChecker
	IDGen          func() (string, error)
	Logger         *slog.Logger
}) *PublishHandler {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	return &PublishHandler{
		templatesWrite: opts.TemplatesWrite,
		accessChecker:  opts.AccessChecker,
		idGen:          opts.IDGen,
		log:            opts.Logger.With("usecase", "publish_template"),
	}
}

func (h *PublishHandler) Execute(ctx context.Context, input PublishInput) (*PublishResult, error) {
	if err := h.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, "template.write"); err != nil {
		if errors.Is(err, domain.ErrWriteDenied) {
			return nil, err
		}
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, domain.ErrWriteDenied
		}
		return nil, err
	}

	tmpl, err := h.templatesWrite.FindTemplateByID(ctx, input.WorkspaceID, input.TemplateID)
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

	if err := shared.ValidateTemplateSource(tmpl.Subject, tmpl.SourceHTML, tmpl.SourceText); err != nil {
		return nil, err
	}

	versions, _, err := h.templatesWrite.ListTemplateVersions(ctx, input.WorkspaceID, input.TemplateID, "", 1)
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
		WorkspaceID:   input.WorkspaceID,
		TemplateID:    input.TemplateID,
		VersionNumber: nextVersion,
		Subject:       tmpl.Subject,
		SourceHTML:    tmpl.SourceHTML,
		SourceText:    tmpl.SourceText,
		Metadata:      tmpl.Metadata,
		PublishedAt:   input.Now,
		CreatedAt:     input.Now,
	}

	tmpl.Status = domain.TemplateStatusActive
	tmpl.CurrentVersionID = verID
	tmpl.UpdatedAt = input.Now

	if err := h.templatesWrite.PublishTemplateVersion(ctx, *tmpl, version); err != nil {
		h.log.Error("failed to publish template version", "error", err)
		return nil, err
	}

	h.log.Info("template published", "template_id", input.TemplateID, "version_id", verID, "version_number", nextVersion)
	return &PublishResult{Version: version}, nil
}
