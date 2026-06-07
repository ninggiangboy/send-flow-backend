package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	contentapp "github.com/ninggiangboy/send-flow/backend/internal/modules/content/app"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/content/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/content/ports"
)

type contentHTTP struct {
	svc *contentapp.Service
}

func newContentHTTP(svc *contentapp.Service) *contentHTTP {
	return &contentHTTP{svc: svc}
}

func (h *contentHTTP) listTemplates(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	userID, _ := r.Context().Value(ctxUserID).(string)

	q := r.URL.Query()
	limit := parseIntParam(q.Get("limit"), 50)
	if limit > 100 {
		limit = 100
	}

	query := ports.TemplateListQuery{
		WorkspaceID: workspaceID,
		Status:      q.Get("status"),
		Q:           q.Get("q"),
		Cursor:      q.Get("cursor"),
		Limit:       limit,
	}

	result, err := h.svc.ListTemplates(r.Context(), query, userID)
	if err != nil {
		writeContentErr(w, r, err)
		return
	}

	out := make([]map[string]any, 0, len(result.Templates))
	for _, t := range result.Templates {
		out = append(out, templateListResponse(t))
	}

	if result.NextCursor != "" {
		w.Header().Set("X-Next-Cursor", result.NextCursor)
	}
	writeEnvelope(w, r, http.StatusOK, out)
}

func (h *contentHTTP) createTemplate(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	userID, _ := r.Context().Value(ctxUserID).(string)

	var req struct {
		Name     string         `json:"name"`
		Subject  string         `json:"subject"`
		HTML     string         `json:"html"`
		Text     string         `json:"text"`
		Metadata map[string]any `json:"metadata"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	result, err := h.svc.CreateTemplate(r.Context(), contentapp.CreateTemplateInput{
		WorkspaceID: workspaceID,
		UserID:      userID,
		Name:        req.Name,
		Subject:     req.Subject,
		SourceHTML:  req.HTML,
		SourceText:  req.Text,
		Metadata:    req.Metadata,
		Now:         time.Now().UTC(),
	})
	if err != nil {
		writeContentErr(w, r, err)
		return
	}

	writeEnvelope(w, r, http.StatusCreated, templateDetailResponse(result.Template))
}

func (h *contentHTTP) getTemplate(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	templateID := chi.URLParam(r, "template_id")
	userID, _ := r.Context().Value(ctxUserID).(string)

	result, err := h.svc.GetTemplate(r.Context(), workspaceID, templateID, userID)
	if err != nil {
		writeContentErr(w, r, err)
		return
	}

	writeEnvelope(w, r, http.StatusOK, templateDetailResponse(result.Template))
}

func (h *contentHTTP) updateTemplate(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	templateID := chi.URLParam(r, "template_id")
	userID, _ := r.Context().Value(ctxUserID).(string)

	var req struct {
		Name     *string        `json:"name"`
		Subject  *string        `json:"subject"`
		HTML     *string        `json:"html"`
		Text     *string        `json:"text"`
		Metadata map[string]any `json:"metadata"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	input := contentapp.UpdateTemplateInput{
		WorkspaceID: workspaceID,
		TemplateID:  templateID,
		UserID:      userID,
		Name:        req.Name,
		Subject:     req.Subject,
		SourceHTML:  req.HTML,
		SourceText:  req.Text,
		Metadata:    req.Metadata,
		Now:         time.Now().UTC(),
	}

	result, err := h.svc.UpdateTemplate(r.Context(), input)
	if err != nil {
		writeContentErr(w, r, err)
		return
	}

	writeEnvelope(w, r, http.StatusOK, templateDetailResponse(result.Template))
}

func (h *contentHTTP) publishTemplate(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	templateID := chi.URLParam(r, "template_id")
	userID, _ := r.Context().Value(ctxUserID).(string)

	result, err := h.svc.PublishTemplate(r.Context(), workspaceID, templateID, userID, time.Now().UTC())
	if err != nil {
		writeContentErr(w, r, err)
		return
	}

	writeEnvelope(w, r, http.StatusOK, templateVersionResponse(result.Version))
}

func (h *contentHTTP) listTemplateVersions(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	templateID := chi.URLParam(r, "template_id")
	userID, _ := r.Context().Value(ctxUserID).(string)

	q := r.URL.Query()
	limit := parseIntParam(q.Get("limit"), 50)
	if limit > 100 {
		limit = 100
	}

	result, err := h.svc.ListTemplateVersions(r.Context(), workspaceID, templateID, userID, limit, q.Get("cursor"))
	if err != nil {
		writeContentErr(w, r, err)
		return
	}

	out := make([]map[string]any, 0, len(result.Versions))
	for _, v := range result.Versions {
		out = append(out, templateVersionResponse(v))
	}

	if result.NextCursor != "" {
		w.Header().Set("X-Next-Cursor", result.NextCursor)
	}
	writeEnvelope(w, r, http.StatusOK, out)
}

func (h *contentHTTP) previewTemplate(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	templateID := chi.URLParam(r, "template_id")
	userID, _ := r.Context().Value(ctxUserID).(string)

	var req struct {
		TemplateData map[string]any `json:"template_data"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	result, err := h.svc.PreviewTemplate(r.Context(), workspaceID, templateID, userID, req.TemplateData, time.Now().UTC())
	if err != nil {
		writeContentErr(w, r, err)
		return
	}

	writeEnvelope(w, r, http.StatusOK, renderResultResponse(result.Result))
}

func (h *contentHTTP) render(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	userID, _ := r.Context().Value(ctxUserID).(string)

	var req struct {
		TemplateID   string         `json:"template_id"`
		Subject      string         `json:"subject"`
		HTML         string         `json:"html"`
		Text         string         `json:"text"`
		TemplateData map[string]any `json:"template_data"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	result, err := h.svc.Render(r.Context(), workspaceID, userID, req.TemplateID, req.Subject, req.HTML, req.Text, req.TemplateData, time.Now().UTC())
	if err != nil {
		writeContentErr(w, r, err)
		return
	}

	resp := renderResultResponse(result.Result)
	if result.Snapshot != nil {
		resp["snapshot_id"] = result.Snapshot.ID
	}

	writeEnvelope(w, r, http.StatusOK, resp)
}

// --- Error mapping ---

func writeContentErr(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrReadDenied):
		writeError(w, r, http.StatusForbidden, "template.read_denied", err.Error(), nil)
	case errors.Is(err, domain.ErrWriteDenied):
		writeError(w, r, http.StatusForbidden, "template.write_denied", err.Error(), nil)
	case errors.Is(err, domain.ErrRenderDenied):
		writeError(w, r, http.StatusForbidden, "template.render_denied", err.Error(), nil)
	case errors.Is(err, domain.ErrTemplateNotFound):
		writeError(w, r, http.StatusNotFound, "template.not_found", err.Error(), nil)
	case errors.Is(err, domain.ErrTemplateVersionNotFound):
		writeError(w, r, http.StatusNotFound, "template.version_not_found", err.Error(), nil)
	case errors.Is(err, domain.ErrPublishConflict):
		writeError(w, r, http.StatusConflict, "template.publish_conflict", err.Error(), nil)
	case errors.Is(err, domain.ErrVersionConflict):
		writeError(w, r, http.StatusConflict, "template.version_conflict", err.Error(), nil)
	case errors.Is(err, domain.ErrSourceInvalid):
		writeError(w, r, http.StatusUnprocessableEntity, "template.source_invalid", err.Error(), nil)
	case errors.Is(err, domain.ErrPublishPayloadInvalid):
		writeError(w, r, http.StatusUnprocessableEntity, "template.publish_payload_invalid", err.Error(), nil)
	case errors.Is(err, domain.ErrRenderPayloadInvalid):
		writeError(w, r, http.StatusUnprocessableEntity, "template.render_payload_invalid", err.Error(), nil)
	case errors.Is(err, domain.ErrRenderContextInvalid):
		writeError(w, r, http.StatusUnprocessableEntity, "template.render_context_invalid", err.Error(), nil)
	default:
		writeError(w, r, http.StatusInternalServerError, "health.runtime_not_ready", "internal error", nil)
	}
}

// --- Response mappers ---

func templateListResponse(t domain.Template) map[string]any {
	return map[string]any{
		"id":                 t.ID,
		"workspace_id":       t.WorkspaceID,
		"name":               t.Name,
		"status":             string(t.Status),
		"current_version_id": t.CurrentVersionID,
		"created_at":         t.CreatedAt,
		"updated_at":         t.UpdatedAt,
	}
}

func templateDetailResponse(t domain.Template) map[string]any {
	return map[string]any{
		"id":                 t.ID,
		"workspace_id":       t.WorkspaceID,
		"name":               t.Name,
		"status":             string(t.Status),
		"subject":            t.Subject,
		"source_html":        t.SourceHTML,
		"source_text":        t.SourceText,
		"current_version_id": t.CurrentVersionID,
		"created_at":         t.CreatedAt,
		"updated_at":         t.UpdatedAt,
	}
}

func templateVersionResponse(v domain.TemplateVersion) map[string]any {
	return map[string]any{
		"id":             v.ID,
		"workspace_id":   v.WorkspaceID,
		"template_id":    v.TemplateID,
		"version_number": v.VersionNumber,
		"subject":        v.Subject,
		"source_html":    v.SourceHTML,
		"source_text":    v.SourceText,
		"published_at":   v.PublishedAt,
		"created_at":     v.CreatedAt,
	}
}

func renderResultResponse(r domain.RenderResult) map[string]any {
	return map[string]any{
		"subject":  r.Subject,
		"html":     r.HTML,
		"text":     r.Text,
		"warnings": r.Warnings,
	}
}
