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

type UpdateInput struct {
	WorkspaceID string
	TemplateID  string
	UserID      string
	Name        *string
	Subject     *string
	SourceHTML  *string
	SourceText  *string
	Metadata    map[string]any
	Now         time.Time
}

type UpdateResult struct {
	Template domain.Template
}

type UpdateHandler struct {
	templatesWrite ports.TemplateWriteRepository
	accessChecker  ports.WorkspaceAccessChecker
	log            *slog.Logger
}

func NewUpdateHandler(opts struct {
	TemplatesWrite ports.TemplateWriteRepository
	AccessChecker  ports.WorkspaceAccessChecker
	Logger         *slog.Logger
}) *UpdateHandler {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	return &UpdateHandler{
		templatesWrite: opts.TemplatesWrite,
		accessChecker:  opts.AccessChecker,
		log:            opts.Logger.With("usecase", "update_template"),
	}
}

func (h *UpdateHandler) Execute(ctx context.Context, input UpdateInput) (*UpdateResult, error) {
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
		h.log.Error("failed to find template", "error", err)
		return nil, err
	}

	if tmpl.Status == domain.TemplateStatusArchived {
		return nil, domain.ErrPublishConflict
	}

	if input.Name != nil {
		tmpl.Name = *input.Name
	}
	subjectChanged := false
	if input.Subject != nil {
		tmpl.Subject = *input.Subject
		subjectChanged = true
	}
	htmlChanged := false
	if input.SourceHTML != nil {
		tmpl.SourceHTML = *input.SourceHTML
		htmlChanged = true
	}
	textChanged := false
	if input.SourceText != nil {
		tmpl.SourceText = *input.SourceText
		textChanged = true
	}

	if subjectChanged || htmlChanged || textChanged {
		if err := shared.ValidateTemplateSource(tmpl.Subject, tmpl.SourceHTML, tmpl.SourceText); err != nil {
			return nil, err
		}
	}

	if input.Metadata != nil {
		tmpl.Metadata = input.Metadata
	}
	tmpl.UpdatedAt = input.Now

	if err := h.templatesWrite.UpdateTemplate(ctx, *tmpl); err != nil {
		h.log.Error("failed to update template", "error", err)
		return nil, err
	}

	h.log.Info("template updated")
	return &UpdateResult{Template: *tmpl}, nil
}
