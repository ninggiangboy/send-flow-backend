package createtemplate

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
	TemplatesWrite ports.TemplateWriteRepository
	AccessChecker  ports.WorkspaceAccessChecker
	IDGen          func() (string, error)
	Logger         *slog.Logger
}

type Handler struct {
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
		templatesWrite: opts.TemplatesWrite,
		accessChecker:  opts.AccessChecker,
		idGen:          opts.IDGen,
		log:            opts.Logger.With("usecase", "create_template"),
	}
}

type Input struct {
	WorkspaceID string
	UserID      string
	Name        string
	Subject     string
	SourceHTML  string
	SourceText  string
	Metadata    map[string]any
	Now         time.Time
}

type Result struct {
	Template domain.Template
}

func (h *Handler) Execute(ctx context.Context, input Input) (*Result, error) {
	if err := h.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, "template.write"); err != nil {
		if errors.Is(err, domain.ErrWriteDenied) {
			return nil, err
		}
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, domain.ErrWriteDenied
		}
		return nil, err
	}

	if input.Name == "" || input.Subject == "" || input.SourceHTML == "" {
		return nil, domain.ErrSourceInvalid
	}

	if err := usecase.ValidateTemplateSource(input.Subject, input.SourceHTML, input.SourceText); err != nil {
		return nil, err
	}

	id, err := h.idGen()
	if err != nil {
		h.log.Error("failed to generate id", "error", err)
		return nil, err
	}

	tmpl := domain.Template{
		ID:          id,
		WorkspaceID: input.WorkspaceID,
		Name:        input.Name,
		Status:      domain.TemplateStatusDraft,
		Subject:     input.Subject,
		SourceHTML:  input.SourceHTML,
		SourceText:  input.SourceText,
		Metadata:    input.Metadata,
		CreatedAt:   input.Now,
		UpdatedAt:   input.Now,
	}

	if err := h.templatesWrite.CreateTemplate(ctx, tmpl); err != nil {
		h.log.Error("failed to create template", "error", err)
		return nil, err
	}

	h.log.Info("template created", "template_id", id)
	return &Result{Template: tmpl}, nil
}
