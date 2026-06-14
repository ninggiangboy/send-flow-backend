package render

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
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
		log:            opts.Logger.With("usecase", "render"),
	}
}

type Query struct {
	WorkspaceID  string
	UserID       string
	TemplateID   string
	Subject      string
	SourceHTML   string
	SourceText   string
	TemplateData map[string]any
	Now          time.Time
}

type Result struct {
	Result   domain.RenderResult
	Snapshot *domain.RenderedTemplateSnapshot
}

func (h *Handler) Execute(ctx context.Context, q Query) (*Result, error) {
	if err := h.accessChecker.RequirePermission(ctx, q.WorkspaceID, q.UserID, "template.render"); err != nil {
		if errors.Is(err, domain.ErrRenderDenied) {
			return nil, err
		}
		if errors.Is(err, domain.ErrReadDenied) {
			return nil, domain.ErrRenderDenied
		}
		return nil, err
	}

	if q.TemplateData == nil {
		return nil, domain.ErrRenderPayloadInvalid
	}

	var result *Result

	if q.TemplateID != "" {
		_, err := h.templatesRead.FindTemplateByID(ctx, q.WorkspaceID, q.TemplateID)
		if err != nil {
			if errors.Is(err, domain.ErrTemplateNotFound) {
				return nil, err
			}
			h.log.Error("failed to find template for render", "error", err)
			return nil, err
		}

		currentVersion, err := h.templatesRead.FindCurrentVersion(ctx, q.WorkspaceID, q.TemplateID)
		if err != nil {
			if errors.Is(err, domain.ErrTemplateVersionNotFound) {
				return nil, err
			}
			h.log.Error("failed to find current version for render", "error", err)
			return nil, err
		}

		rendered, err := usecase.RenderTemplateSource(currentVersion.Subject, currentVersion.SourceHTML, currentVersion.SourceText, q.TemplateData)
		if err != nil {
			return nil, err
		}
		result = &Result{Result: *rendered}

		snapshot := h.makeSnapshot(q.WorkspaceID, q.TemplateID, currentVersion.ID, q.TemplateData, rendered, q.Now)
		if snapshot != nil {
			if err := h.templatesWrite.CreateRenderSnapshot(ctx, *snapshot); err != nil {
				h.log.Warn("failed to save render snapshot", "error", err)
			}
		}
		result.Snapshot = snapshot
	} else if q.Subject != "" || q.SourceHTML != "" {
		rendered, err := usecase.RenderTemplateSource(q.Subject, q.SourceHTML, q.SourceText, q.TemplateData)
		if err != nil {
			return nil, err
		}
		result = &Result{Result: *rendered}
	} else {
		return nil, domain.ErrRenderPayloadInvalid
	}

	return result, nil
}

func (h *Handler) makeSnapshot(workspaceID, templateID, versionID string, input map[string]any, rendered *domain.RenderResult, now time.Time) *domain.RenderedTemplateSnapshot {
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return nil
	}
	hash := sha256.Sum256(inputJSON)
	hashStr := fmt.Sprintf("%x", hash)

	id, err := h.idGen()
	if err != nil {
		return nil
	}

	return &domain.RenderedTemplateSnapshot{
		ID:                id,
		WorkspaceID:       workspaceID,
		TemplateID:        templateID,
		TemplateVersionID: versionID,
		RenderInputHash:   hashStr,
		Subject:           rendered.Subject,
		RenderedHTML:      rendered.HTML,
		RenderedText:      rendered.Text,
		Warnings:          rendered.Warnings,
		CreatedAt:         now,
	}
}
