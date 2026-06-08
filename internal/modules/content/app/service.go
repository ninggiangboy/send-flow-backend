package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"strings"
	"time"

	texttemplate "text/template"

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

type Service struct {
	templatesRead  ports.TemplateReadRepository
	templatesWrite ports.TemplateWriteRepository
	accessChecker  ports.WorkspaceAccessChecker
	idGen          func() (string, error)
	log            *slog.Logger
}

func NewService(opts Options) *Service {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.IDGen == nil {
		opts.IDGen = func() (string, error) { return "", nil }
	}
	return &Service{
		templatesRead:  opts.TemplatesRead,
		templatesWrite: opts.TemplatesWrite,
		accessChecker:  opts.AccessChecker,
		idGen:          opts.IDGen,
		log:            opts.Logger.With("module", "content"),
	}
}

type CreateTemplateInput struct {
	WorkspaceID string
	UserID      string
	Name        string
	Subject     string
	SourceHTML  string
	SourceText  string
	Metadata    map[string]any
	Now         time.Time
}

type UpdateTemplateInput struct {
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

type TemplateResult struct {
	Template domain.Template
}

type TemplateListResult struct {
	Templates  []domain.Template
	NextCursor string
}

type VersionResult struct {
	Version domain.TemplateVersion
}

type VersionListResult struct {
	Versions   []domain.TemplateVersion
	NextCursor string
}

type RenderResult struct {
	Result   domain.RenderResult
	Snapshot *domain.RenderedTemplateSnapshot
}

func (s *Service) CreateTemplate(ctx context.Context, input CreateTemplateInput) (*TemplateResult, error) {
	log := s.log.With("usecase", "create_template", "workspace_id", input.WorkspaceID)
	if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, "template.write"); err != nil {
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

	if err := validateTemplateSource(input.Subject, input.SourceHTML, input.SourceText); err != nil {
		return nil, err
	}

	id, err := s.idGen()
	if err != nil {
		log.Error("failed to generate id", "error", err)
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

	if err := s.templatesWrite.CreateTemplate(ctx, tmpl); err != nil {
		log.Error("failed to create template", "error", err)
		return nil, err
	}

	log.Info("template created", "template_id", id)
	return &TemplateResult{Template: tmpl}, nil
}

func (s *Service) ListTemplates(ctx context.Context, query ports.TemplateListQuery, userID string) (*TemplateListResult, error) {
	log := s.log.With("usecase", "list_templates", "workspace_id", query.WorkspaceID)
	if err := s.accessChecker.RequirePermission(ctx, query.WorkspaceID, userID, "template.read"); err != nil {
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, err
		}
		return nil, err
	}

	if query.Limit <= 0 || query.Limit > 100 {
		query.Limit = 50
	}

	templates, cursor, err := s.templatesRead.ListTemplates(ctx, query)
	if err != nil {
		log.Error("failed to list templates", "error", err)
		return nil, err
	}

	return &TemplateListResult{Templates: templates, NextCursor: cursor}, nil
}

func (s *Service) GetTemplate(ctx context.Context, workspaceID, templateID, userID string) (*TemplateResult, error) {
	log := s.log.With("usecase", "get_template", "workspace_id", workspaceID, "template_id", templateID)
	if err := s.accessChecker.RequirePermission(ctx, workspaceID, userID, "template.read"); err != nil {
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, err
		}
		return nil, err
	}

	tmpl, err := s.templatesRead.FindTemplateByID(ctx, workspaceID, templateID)
	if err != nil {
		if errors.Is(err, domain.ErrTemplateNotFound) {
			return nil, err
		}
		log.Error("failed to find template", "error", err)
		return nil, err
	}

	return &TemplateResult{Template: *tmpl}, nil
}

func (s *Service) UpdateTemplate(ctx context.Context, input UpdateTemplateInput) (*TemplateResult, error) {
	log := s.log.With("usecase", "update_template", "workspace_id", input.WorkspaceID, "template_id", input.TemplateID)
	if err := s.accessChecker.RequirePermission(ctx, input.WorkspaceID, input.UserID, "template.write"); err != nil {
		if errors.Is(err, domain.ErrWriteDenied) {
			return nil, err
		}
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, domain.ErrWriteDenied
		}
		return nil, err
	}

	tmpl, err := s.templatesRead.FindTemplateByID(ctx, input.WorkspaceID, input.TemplateID)
	if err != nil {
		if errors.Is(err, domain.ErrTemplateNotFound) {
			return nil, err
		}
		log.Error("failed to find template", "error", err)
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
		if err := validateTemplateSource(tmpl.Subject, tmpl.SourceHTML, tmpl.SourceText); err != nil {
			return nil, err
		}
	}

	if input.Metadata != nil {
		tmpl.Metadata = input.Metadata
	}
	tmpl.UpdatedAt = input.Now

	if err := s.templatesWrite.UpdateTemplate(ctx, *tmpl); err != nil {
		log.Error("failed to update template", "error", err)
		return nil, err
	}

	log.Info("template updated")
	return &TemplateResult{Template: *tmpl}, nil
}

func (s *Service) PublishTemplate(ctx context.Context, workspaceID, templateID, userID string, now time.Time) (*VersionResult, error) {
	log := s.log.With("usecase", "publish_template", "workspace_id", workspaceID, "template_id", templateID)
	if err := s.accessChecker.RequirePermission(ctx, workspaceID, userID, "template.write"); err != nil {
		if errors.Is(err, domain.ErrWriteDenied) {
			return nil, err
		}
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, domain.ErrWriteDenied
		}
		return nil, err
	}

	tmpl, err := s.templatesRead.FindTemplateByID(ctx, workspaceID, templateID)
	if err != nil {
		if errors.Is(err, domain.ErrTemplateNotFound) {
			return nil, err
		}
		log.Error("failed to find template for publish", "error", err)
		return nil, err
	}

	if tmpl.Status == domain.TemplateStatusArchived {
		return nil, domain.ErrPublishConflict
	}

	if err := validateTemplateSource(tmpl.Subject, tmpl.SourceHTML, tmpl.SourceText); err != nil {
		return nil, err
	}

	versions, _, err := s.templatesRead.ListTemplateVersions(ctx, ports.VersionListQuery{
		WorkspaceID: workspaceID,
		TemplateID:  templateID,
		Limit:       1,
	})
	if err != nil {
		log.Error("failed to list versions for version number", "error", err)
		return nil, err
	}

	nextVersion := 1
	if len(versions) > 0 {
		nextVersion = versions[0].VersionNumber + 1
	}

	verID, err := s.idGen()
	if err != nil {
		log.Error("failed to generate id", "error", err)
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

	if err := s.templatesWrite.PublishTemplateVersion(ctx, *tmpl, version); err != nil {
		log.Error("failed to publish template version", "error", err)
		return nil, err
	}

	log.Info("template published", "template_id", templateID, "version_id", verID, "version_number", nextVersion)
	return &VersionResult{Version: version}, nil
}

func (s *Service) ListTemplateVersions(ctx context.Context, workspaceID, templateID, userID string, limit int, cursor string) (*VersionListResult, error) {
	log := s.log.With("usecase", "list_template_versions", "workspace_id", workspaceID, "template_id", templateID)
	if err := s.accessChecker.RequirePermission(ctx, workspaceID, userID, "template.read"); err != nil {
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, err
		}
		return nil, err
	}

	if limit <= 0 || limit > 100 {
		limit = 50
	}

	query := ports.VersionListQuery{WorkspaceID: workspaceID, TemplateID: templateID, Limit: limit, Cursor: cursor}
	versions, nextCursor, err := s.templatesRead.ListTemplateVersions(ctx, query)
	if err != nil {
		log.Error("failed to list template versions", "error", err)
		return nil, err
	}

	return &VersionListResult{Versions: versions, NextCursor: nextCursor}, nil
}

func (s *Service) PreviewTemplate(ctx context.Context, workspaceID, templateID, userID string, templateData map[string]any, now time.Time) (*RenderResult, error) {
	log := s.log.With("usecase", "preview_template", "workspace_id", workspaceID, "template_id", templateID)
	if err := s.accessChecker.RequirePermission(ctx, workspaceID, userID, "template.render"); err != nil {
		if errors.Is(err, domain.ErrRenderDenied) {
			return nil, err
		}
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, domain.ErrRenderDenied
		}
		return nil, err
	}

	tmpl, err := s.templatesRead.FindTemplateByID(ctx, workspaceID, templateID)
	if err != nil {
		if errors.Is(err, domain.ErrTemplateNotFound) {
			return nil, err
		}
		log.Error("failed to find template for preview", "error", err)
		return nil, err
	}

	r, err := renderTemplateSource(tmpl.Subject, tmpl.SourceHTML, tmpl.SourceText, templateData)
	if err != nil {
		return nil, err
	}

	return &RenderResult{Result: *r}, nil
}

func (s *Service) Render(ctx context.Context, workspaceID, userID, templateID, subject, sourceHTML, sourceText string, templateData map[string]any, now time.Time) (*RenderResult, error) {
	log := s.log.With("usecase", "render", "workspace_id", workspaceID)
	if err := s.accessChecker.RequirePermission(ctx, workspaceID, userID, "template.render"); err != nil {
		if errors.Is(err, domain.ErrRenderDenied) {
			return nil, err
		}
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, domain.ErrRenderDenied
		}
		return nil, err
	}

	if templateData == nil {
		return nil, domain.ErrRenderPayloadInvalid
	}

	var result *RenderResult
	var snapshot *domain.RenderedTemplateSnapshot

	if templateID != "" {
		_, err := s.templatesRead.FindTemplateByID(ctx, workspaceID, templateID)
		if err != nil {
			if errors.Is(err, domain.ErrTemplateNotFound) {
				return nil, err
			}
			log.Error("failed to find template for render", "error", err)
			return nil, err
		}

		currentVersion, err := s.templatesRead.FindCurrentVersion(ctx, workspaceID, templateID)
		if err != nil {
			if errors.Is(err, domain.ErrTemplateVersionNotFound) {
				return nil, err
			}
			log.Error("failed to find current version for render", "error", err)
			return nil, err
		}

		rendered, err := renderTemplateSource(currentVersion.Subject, currentVersion.SourceHTML, currentVersion.SourceText, templateData)
		if err != nil {
			return nil, err
		}
		result = &RenderResult{Result: *rendered}

		snapshot = s.makeSnapshot(workspaceID, templateID, currentVersion.ID, templateData, result, now)
		if snapshot != nil {
			if err := s.templatesWrite.CreateRenderSnapshot(ctx, *snapshot); err != nil {
				log.Warn("failed to save render snapshot", "error", err)
			}
		}
		result.Snapshot = snapshot
	} else if subject != "" || sourceHTML != "" {
		rendered, err := renderTemplateSource(subject, sourceHTML, sourceText, templateData)
		if err != nil {
			return nil, err
		}
		result = &RenderResult{Result: *rendered}
	} else {
		return nil, domain.ErrRenderPayloadInvalid
	}

	return result, nil
}

func (s *Service) makeSnapshot(workspaceID, templateID, versionID string, input map[string]any, result *RenderResult, now time.Time) *domain.RenderedTemplateSnapshot {
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return nil
	}
	hash := sha256.Sum256(inputJSON)
	hashStr := fmt.Sprintf("%x", hash)

	id, err := s.idGen()
	if err != nil {
		return nil
	}

	return &domain.RenderedTemplateSnapshot{
		ID:                id,
		WorkspaceID:       workspaceID,
		TemplateID:        templateID,
		TemplateVersionID: versionID,
		RenderInputHash:   hashStr,
		Subject:           result.Result.Subject,
		RenderedHTML:      result.Result.HTML,
		RenderedText:      result.Result.Text,
		Warnings:          result.Result.Warnings,
		CreatedAt:         now,
	}
}

func validateTemplateSource(subject, sourceHTML, sourceText string) error {
	if _, err := texttemplate.New("subject").Option("missingkey=error").Parse(subject); err != nil {
		return fmt.Errorf("%w: subject: %v", domain.ErrSourceInvalid, err)
	}
	if _, err := template.New("html").Option("missingkey=error").Parse(sourceHTML); err != nil {
		return fmt.Errorf("%w: html: %v", domain.ErrSourceInvalid, err)
	}
	if sourceText != "" {
		if _, err := texttemplate.New("text").Option("missingkey=error").Parse(sourceText); err != nil {
			return fmt.Errorf("%w: text: %v", domain.ErrSourceInvalid, err)
		}
	}
	return nil
}

func renderTemplateSource(subject, sourceHTML, sourceText string, data map[string]any) (*domain.RenderResult, error) {
	var warnings []string

	subjectTmpl, err := texttemplate.New("subject").Option("missingkey=error").Parse(subject)
	if err != nil {
		return nil, fmt.Errorf("%w: subject parse: %v", domain.ErrSourceInvalid, err)
	}
	htmlTmpl, err := template.New("html").Option("missingkey=error").Parse(strings.TrimSpace(sourceHTML))
	if err != nil {
		return nil, fmt.Errorf("%w: html parse: %v", domain.ErrSourceInvalid, err)
	}

	var subjectBuf bytes.Buffer
	if err := subjectTmpl.Execute(&subjectBuf, data); err != nil {
		if isMissingKeyError(err) {
			return nil, fmt.Errorf("%w: subject missing key: %v", domain.ErrRenderPayloadInvalid, err)
		}
		return nil, fmt.Errorf("%w: subject execute: %v", domain.ErrRenderContextInvalid, err)
	}

	var htmlBuf bytes.Buffer
	if err := htmlTmpl.Execute(&htmlBuf, data); err != nil {
		if isMissingKeyError(err) {
			return nil, fmt.Errorf("%w: html missing key: %v", domain.ErrRenderPayloadInvalid, err)
		}
		return nil, fmt.Errorf("%w: html execute: %v", domain.ErrRenderContextInvalid, err)
	}

	var textResult string
	if sourceText != "" {
		textTmpl, err := texttemplate.New("text").Option("missingkey=error").Parse(sourceText)
		if err != nil {
			return nil, fmt.Errorf("%w: text parse: %v", domain.ErrSourceInvalid, err)
		}
		var textBuf bytes.Buffer
		if err := textTmpl.Execute(&textBuf, data); err != nil {
			if isMissingKeyError(err) {
				return nil, fmt.Errorf("%w: text missing key: %v", domain.ErrRenderPayloadInvalid, err)
			}
			return nil, fmt.Errorf("%w: text execute: %v", domain.ErrRenderContextInvalid, err)
		}
		textResult = textBuf.String()
	}

	return &domain.RenderResult{
		Subject:  subjectBuf.String(),
		HTML:     htmlBuf.String(),
		Text:     textResult,
		Warnings: warnings,
	}, nil
}

func isMissingKeyError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "missing value") || strings.Contains(msg, "<nil>") || strings.Contains(msg, "map has no entry for key")
}

func (s *Service) RenderVersion(ctx context.Context, workspaceID, templateVersionID string, data map[string]any) (*RenderResult, error) {
	log := s.log.With("usecase", "render_version", "workspace_id", workspaceID, "template_version_id", templateVersionID)

	if data == nil {
		data = map[string]any{}
	}

	version, err := s.templatesRead.FindTemplateVersionByID(ctx, workspaceID, templateVersionID)
	if err != nil {
		if errors.Is(err, domain.ErrTemplateVersionNotFound) {
			return nil, err
		}
		log.Error("failed to find template version for render", "error", err)
		return nil, err
	}

	rendered, err := renderTemplateSource(version.Subject, version.SourceHTML, version.SourceText, data)
	if err != nil {
		return nil, err
	}

	return &RenderResult{Result: *rendered}, nil
}

func (s *Service) GetPublishedTemplateVersion(ctx context.Context, workspaceID, templateID string) (*domain.TemplateVersion, error) {
	log := s.log.With("usecase", "get_published_template_version", "workspace_id", workspaceID, "template_id", templateID)

	version, err := s.templatesRead.FindCurrentVersion(ctx, workspaceID, templateID)
	if err != nil {
		if err == domain.ErrTemplateVersionNotFound {
			return nil, err
		}
		log.Error("failed to find current version", "error", err)
		return nil, err
	}
	return version, nil
}

func (s *Service) ValidateTemplateRenderable(ctx context.Context, workspaceID, templateID string) error {
	log := s.log.With("usecase", "validate_template_renderable", "workspace_id", workspaceID, "template_id", templateID)

	tmpl, err := s.templatesRead.FindTemplateByID(ctx, workspaceID, templateID)
	if err != nil {
		if err == domain.ErrTemplateNotFound {
			return err
		}
		log.Error("failed to find template", "error", err)
		return err
	}

	if err := validateTemplateSource(tmpl.Subject, tmpl.SourceHTML, tmpl.SourceText); err != nil {
		return err
	}

	return nil
}
