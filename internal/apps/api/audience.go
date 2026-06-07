package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	audienceapp "github.com/ninggiangboy/send-flow/backend/internal/modules/audience/app"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/audience/ports"
)

type audienceHTTP struct {
	svc *audienceapp.Service
}

func newAudienceHTTP(svc *audienceapp.Service) *audienceHTTP {
	return &audienceHTTP{svc: svc}
}

func (h *audienceHTTP) listContacts(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	userID, _ := r.Context().Value(ctxUserID).(string)

	q := r.URL.Query()
	limit := parseIntParam(q.Get("limit"), 50)
	if limit > 100 {
		limit = 100
	}

	query := ports.ContactListQuery{
		WorkspaceID: workspaceID,
		Q:           q.Get("q"),
		Status:      q.Get("status"),
		ListID:      q.Get("list_id"),
		SegmentID:   q.Get("segment_id"),
		Limit:       limit,
		Cursor:      q.Get("cursor"),
		Sort:        q.Get("sort"),
	}

	result, err := h.svc.ListContacts(r.Context(), query, userID)
	if err != nil {
		writeAudienceErr(w, r, err)
		return
	}

	out := make([]map[string]any, 0, len(result.Contacts))
	for _, c := range result.Contacts {
		out = append(out, contactResponse(c))
	}

	env := envelope{Data: out, Meta: meta{RequestID: requestID(r)}}
	if result.NextCursor != "" {
		env.Meta = meta{RequestID: requestID(r)}
	}

	w.Header().Set("Content-Type", "application/json")
	if result.NextCursor != "" {
		w.Header().Set("X-Next-Cursor", result.NextCursor)
	}
	writeEnvelope(w, r, http.StatusOK, out)
}

func (h *audienceHTTP) createContact(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	userID, _ := r.Context().Value(ctxUserID).(string)

	var req struct {
		Email      string         `json:"email"`
		FirstName  string         `json:"first_name"`
		LastName   string         `json:"last_name"`
		Tags       []string       `json:"tags"`
		Attributes map[string]any `json:"attributes"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	result, err := h.svc.CreateContact(r.Context(), audienceapp.CreateContactInput{
		WorkspaceID: workspaceID,
		UserID:      userID,
		Email:       req.Email,
		FirstName:   req.FirstName,
		LastName:    req.LastName,
		Tags:        req.Tags,
		Attributes:  req.Attributes,
		Now:         time.Now().UTC(),
	})
	if err != nil {
		writeAudienceErr(w, r, err)
		return
	}

	writeEnvelope(w, r, http.StatusCreated, contactResponse(result.Contact))
}

func (h *audienceHTTP) getContact(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	contactID := chi.URLParam(r, "contact_id")
	userID, _ := r.Context().Value(ctxUserID).(string)

	result, err := h.svc.GetContact(r.Context(), workspaceID, contactID, userID)
	if err != nil {
		writeAudienceErr(w, r, err)
		return
	}

	writeEnvelope(w, r, http.StatusOK, contactResponse(result.Contact))
}

func (h *audienceHTTP) updateContact(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	contactID := chi.URLParam(r, "contact_id")
	userID, _ := r.Context().Value(ctxUserID).(string)

	var req struct {
		Email      *string        `json:"email"`
		FirstName  *string        `json:"first_name"`
		LastName   *string        `json:"last_name"`
		Status     *string        `json:"status"`
		Tags       []string       `json:"tags"`
		Attributes map[string]any `json:"attributes"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	input := audienceapp.UpdateContactInput{
		WorkspaceID: workspaceID,
		ContactID:   contactID,
		UserID:      userID,
		Email:       req.Email,
		FirstName:   req.FirstName,
		LastName:    req.LastName,
		Status:      req.Status,
		Tags:        req.Tags,
		Attributes:  req.Attributes,
		Now:         time.Now().UTC(),
	}

	result, err := h.svc.UpdateContact(r.Context(), input)
	if err != nil {
		writeAudienceErr(w, r, err)
		return
	}

	writeEnvelope(w, r, http.StatusOK, contactResponse(result.Contact))
}

func (h *audienceHTTP) deleteContact(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	contactID := chi.URLParam(r, "contact_id")
	userID, _ := r.Context().Value(ctxUserID).(string)

	if err := h.svc.ArchiveContact(r.Context(), workspaceID, contactID, userID, time.Now().UTC()); err != nil {
		writeAudienceErr(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *audienceHTTP) listAudienceLists(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	userID, _ := r.Context().Value(ctxUserID).(string)

	q := r.URL.Query()
	limit := parseIntParam(q.Get("limit"), 50)
	if limit > 100 {
		limit = 100
	}

	result, err := h.svc.ListLists(r.Context(), workspaceID, userID, limit, q.Get("cursor"))
	if err != nil {
		writeAudienceErr(w, r, err)
		return
	}

	out := make([]map[string]any, 0, len(result.Lists))
	for _, l := range result.Lists {
		resp := listResponse(l)
		if count, ok := result.Counts[l.ID]; ok {
			resp["contact_count"] = count
		}
		out = append(out, resp)
	}

	if result.NextCursor != "" {
		w.Header().Set("X-Next-Cursor", result.NextCursor)
	}
	writeEnvelope(w, r, http.StatusOK, out)
}

func (h *audienceHTTP) createAudienceList(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	userID, _ := r.Context().Value(ctxUserID).(string)

	var req struct {
		Name        string         `json:"name"`
		Description string         `json:"description"`
		Metadata    map[string]any `json:"metadata"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	result, err := h.svc.CreateList(r.Context(), workspaceID, userID, req.Name, req.Description, req.Metadata, time.Now().UTC())
	if err != nil {
		writeAudienceErr(w, r, err)
		return
	}

	writeEnvelope(w, r, http.StatusCreated, listResponse(result.List))
}

func (h *audienceHTTP) updateAudienceListContacts(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	listID := chi.URLParam(r, "list_id")
	userID, _ := r.Context().Value(ctxUserID).(string)

	var req struct {
		Mode       string   `json:"mode"`
		ContactIDs []string `json:"contact_ids"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	result, err := h.svc.UpdateListMemberships(r.Context(), workspaceID, listID, userID, req.Mode, req.ContactIDs, time.Now().UTC())
	if err != nil {
		writeAudienceErr(w, r, err)
		return
	}

	writeEnvelope(w, r, http.StatusOK, map[string]any{
		"added_count":   result.Result.AddedCount,
		"removed_count": result.Result.RemovedCount,
		"skipped_count": result.Result.SkippedCount,
	})
}

func (h *audienceHTTP) listSegments(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	userID, _ := r.Context().Value(ctxUserID).(string)

	q := r.URL.Query()
	limit := parseIntParam(q.Get("limit"), 50)
	if limit > 100 {
		limit = 100
	}

	result, err := h.svc.ListSegments(r.Context(), workspaceID, userID, q.Get("status"), limit, q.Get("cursor"))
	if err != nil {
		writeAudienceErr(w, r, err)
		return
	}

	out := make([]map[string]any, 0, len(result.Segments))
	for _, s := range result.Segments {
		out = append(out, segmentResponse(s))
	}

	if result.NextCursor != "" {
		w.Header().Set("X-Next-Cursor", result.NextCursor)
	}
	writeEnvelope(w, r, http.StatusOK, out)
}

func (h *audienceHTTP) createSegment(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	userID, _ := r.Context().Value(ctxUserID).(string)

	var req struct {
		Name       string         `json:"name"`
		Definition map[string]any `json:"definition"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	result, err := h.svc.CreateSegment(r.Context(), workspaceID, userID, req.Name, req.Definition, time.Now().UTC())
	if err != nil {
		writeAudienceErr(w, r, err)
		return
	}

	writeEnvelope(w, r, http.StatusCreated, segmentResponse(result.Segment))
}

func (h *audienceHTTP) updateSegment(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	segmentID := chi.URLParam(r, "segment_id")
	userID, _ := r.Context().Value(ctxUserID).(string)

	var req struct {
		Name       string         `json:"name"`
		Definition map[string]any `json:"definition"`
		Status     string         `json:"status"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	result, err := h.svc.UpdateSegment(r.Context(), workspaceID, segmentID, userID, req.Name, req.Definition, req.Status, time.Now().UTC())
	if err != nil {
		writeAudienceErr(w, r, err)
		return
	}

	writeEnvelope(w, r, http.StatusOK, segmentResponse(result.Segment))
}

func (h *audienceHTTP) startAudienceImport(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	userID, _ := r.Context().Value(ctxUserID).(string)

	var req struct {
		SourceURI  string         `json:"source_uri"`
		DedupeMode string         `json:"dedupe_mode"`
		Metadata   map[string]any `json:"metadata"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	result, err := h.svc.StartAudienceImport(r.Context(), workspaceID, userID, req.SourceURI, req.DedupeMode, req.Metadata, time.Now().UTC())
	if err != nil {
		writeAudienceErr(w, r, err)
		return
	}

	writeEnvelope(w, r, http.StatusCreated, importJobResponse(result.Job))
}

func (h *audienceHTTP) listAudienceImports(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	userID, _ := r.Context().Value(ctxUserID).(string)

	q := r.URL.Query()
	limit := parseIntParam(q.Get("limit"), 50)
	if limit > 100 {
		limit = 100
	}

	result, err := h.svc.ListAudienceImports(r.Context(), workspaceID, userID, q.Get("status"), limit, q.Get("cursor"))
	if err != nil {
		writeAudienceErr(w, r, err)
		return
	}

	out := make([]map[string]any, 0, len(result.Jobs))
	for _, j := range result.Jobs {
		out = append(out, importJobResponse(j))
	}

	if result.NextCursor != "" {
		w.Header().Set("X-Next-Cursor", result.NextCursor)
	}
	writeEnvelope(w, r, http.StatusOK, out)
}

func (h *audienceHTTP) getAudienceImport(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	importID := chi.URLParam(r, "import_id")
	userID, _ := r.Context().Value(ctxUserID).(string)

	result, err := h.svc.GetAudienceImport(r.Context(), workspaceID, importID, userID)
	if err != nil {
		writeAudienceErr(w, r, err)
		return
	}

	writeEnvelope(w, r, http.StatusOK, importJobResponse(result.Job))
}

func (h *audienceHTTP) startAudienceExport(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	userID, _ := r.Context().Value(ctxUserID).(string)

	var req struct {
		Format         string         `json:"format"`
		Filters        map[string]any `json:"filters"`
		SelectedFields []string       `json:"selected_fields"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	result, err := h.svc.StartAudienceExport(r.Context(), workspaceID, userID, req.Format, req.Filters, req.SelectedFields, time.Now().UTC())
	if err != nil {
		writeAudienceErr(w, r, err)
		return
	}

	writeEnvelope(w, r, http.StatusCreated, exportJobResponse(result.Job))
}

func (h *audienceHTTP) getAudienceExport(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	exportID := chi.URLParam(r, "export_id")
	userID, _ := r.Context().Value(ctxUserID).(string)

	result, err := h.svc.GetAudienceExport(r.Context(), workspaceID, exportID, userID)
	if err != nil {
		writeAudienceErr(w, r, err)
		return
	}

	writeEnvelope(w, r, http.StatusOK, exportJobResponse(result.Job))
}

// --- Error mapping ---

func writeAudienceErr(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrReadDenied):
		writeError(w, r, http.StatusForbidden, "audience.read_denied", err.Error(), nil)
	case errors.Is(err, domain.ErrWriteDenied):
		writeError(w, r, http.StatusForbidden, "audience.write_denied", err.Error(), nil)
	case errors.Is(err, domain.ErrImportDenied):
		writeError(w, r, http.StatusForbidden, "audience.import_denied", err.Error(), nil)
	case errors.Is(err, domain.ErrExportDenied):
		writeError(w, r, http.StatusForbidden, "audience.export_denied", err.Error(), nil)
	case errors.Is(err, domain.ErrContactNotFound):
		writeError(w, r, http.StatusNotFound, "audience.contact_not_found", err.Error(), nil)
	case errors.Is(err, domain.ErrListNotFound):
		writeError(w, r, http.StatusNotFound, "audience.list_not_found", err.Error(), nil)
	case errors.Is(err, domain.ErrSegmentNotFound):
		writeError(w, r, http.StatusNotFound, "audience.segment_not_found", err.Error(), nil)
	case errors.Is(err, domain.ErrImportJobNotFound):
		writeError(w, r, http.StatusNotFound, "audience.import_job_not_found", err.Error(), nil)
	case errors.Is(err, domain.ErrExportJobNotFound):
		writeError(w, r, http.StatusNotFound, "audience.export_job_not_found", err.Error(), nil)
	case errors.Is(err, domain.ErrContactEmailConflict):
		writeError(w, r, http.StatusConflict, "audience.contact_email_conflict", err.Error(), nil)
	case errors.Is(err, domain.ErrListNameConflict):
		writeError(w, r, http.StatusConflict, "audience.list_name_conflict", err.Error(), nil)
	case errors.Is(err, domain.ErrSegmentNameConflict):
		writeError(w, r, http.StatusConflict, "audience.segment_name_conflict", err.Error(), nil)
	case errors.Is(err, domain.ErrImportDuplicateSubmission):
		writeError(w, r, http.StatusConflict, "audience.import_duplicate_submission", err.Error(), nil)
	case errors.Is(err, domain.ErrContactPayloadInvalid):
		writeError(w, r, http.StatusUnprocessableEntity, "audience.contact_payload_invalid", err.Error(), nil)
	case errors.Is(err, domain.ErrContactStatusInvalid):
		writeError(w, r, http.StatusUnprocessableEntity, "audience.contact_status_invalid", err.Error(), nil)
	case errors.Is(err, domain.ErrSegmentDefinitionInvalid):
		writeError(w, r, http.StatusUnprocessableEntity, "audience.segment_definition_invalid", err.Error(), nil)
	case errors.Is(err, domain.ErrListMembershipPayloadInvalid):
		writeError(w, r, http.StatusUnprocessableEntity, "audience.list_membership_payload_invalid", err.Error(), nil)
	case errors.Is(err, domain.ErrImportSourceInvalid):
		writeError(w, r, http.StatusUnprocessableEntity, "audience.import_source_invalid", err.Error(), nil)
	case errors.Is(err, domain.ErrExportFilterInvalid):
		writeError(w, r, http.StatusUnprocessableEntity, "audience.export_filter_invalid", err.Error(), nil)
	case errors.Is(err, domain.ErrExportFormatInvalid):
		writeError(w, r, http.StatusUnprocessableEntity, "audience.export_format_invalid", err.Error(), nil)
	default:
		writeError(w, r, http.StatusInternalServerError, "health.runtime_not_ready", "internal error", nil)
	}
}

// --- Response mappers ---

func contactResponse(c domain.Contact) map[string]any {
	out := map[string]any{
		"id":           c.ID,
		"workspace_id": c.WorkspaceID,
		"email":        c.Email,
		"status":       string(c.Status),
		"first_name":   c.FirstName,
		"last_name":    c.LastName,
		"tags":         c.Tags,
		"attributes":   c.Attributes,
		"created_at":   c.CreatedAt,
		"updated_at":   c.UpdatedAt,
	}
	return out
}

func listResponse(l domain.AudienceList) map[string]any {
	out := map[string]any{
		"id":           l.ID,
		"workspace_id": l.WorkspaceID,
		"name":         l.Name,
		"description":  l.Description,
		"created_at":   l.CreatedAt,
		"updated_at":   l.UpdatedAt,
	}
	return out
}

func segmentResponse(s domain.Segment) map[string]any {
	out := map[string]any{
		"id":           s.ID,
		"workspace_id": s.WorkspaceID,
		"name":         s.Name,
		"definition":   s.DefinitionJSON,
		"status":       string(s.Status),
		"created_at":   s.CreatedAt,
		"updated_at":   s.UpdatedAt,
	}
	return out
}

func importJobResponse(j domain.AudienceImportJob) map[string]any {
	out := map[string]any{
		"id":              j.ID,
		"workspace_id":    j.WorkspaceID,
		"source_uri":      j.SourceURI,
		"dedupe_mode":     string(j.DedupeMode),
		"status":          string(j.Status),
		"processed_count": j.ProcessedCount,
		"created_count":   j.CreatedCount,
		"updated_count":   j.UpdatedCount,
		"failed_count":    j.FailedCount,
		"error_summary":   j.ErrorSummary,
		"created_at":      j.CreatedAt,
		"updated_at":      j.UpdatedAt,
		"completed_at":    j.CompletedAt,
	}
	return out
}

func exportJobResponse(j domain.AudienceExportJob) map[string]any {
	out := map[string]any{
		"id":            j.ID,
		"workspace_id":  j.WorkspaceID,
		"format":        string(j.Format),
		"status":        string(j.Status),
		"artifact_uri":  j.ArtifactURI,
		"error_summary": j.ErrorSummary,
		"created_at":    j.CreatedAt,
		"updated_at":    j.UpdatedAt,
		"completed_at":  j.CompletedAt,
	}
	return out
}

func parseIntParam(s string, defaultVal int) int {
	if s == "" {
		return defaultVal
	}
	var n int
	for _, c := range s {
		if c < '0' || c > '9' {
			return defaultVal
		}
		n = n*10 + int(c-'0')
	}
	return n
}
