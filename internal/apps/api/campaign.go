package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	campaignapp "github.com/ninggiangboy/send-flow/backend/internal/modules/campaign/app"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/campaign/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/auth"
)

type campaignHTTP struct {
	svc *campaignapp.Service
}

func newCampaignHTTP(svc *campaignapp.Service) *campaignHTTP {
	return &campaignHTTP{svc: svc}
}

func (h *campaignHTTP) listCampaigns(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	userID, _ := r.Context().Value(ctxUserID).(string)

	query := campaignapp.ListInput{
		WorkspaceID: workspaceID,
		Status:      r.URL.Query().Get("status"),
		Cursor:      r.URL.Query().Get("cursor"),
	}

	results, err := h.svc.ListCampaigns(r.Context(), query, userID)
	if err != nil {
		writeCampaignErr(w, r, err)
		return
	}

	out := make([]map[string]any, 0, len(results.Campaigns))
	for _, c := range results.Campaigns {
		out = append(out, campaignResponse(c, 0))
	}
	writeEnvelope(w, r, http.StatusOK, map[string]any{
		"campaigns":   out,
		"next_cursor": results.NextCursor,
	})
}

func (h *campaignHTTP) createCampaign(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	userID, _ := r.Context().Value(ctxUserID).(string)

	var req struct {
		Name           string          `json:"name"`
		AudienceRef    *audienceRefDTO `json:"audience_ref"`
		TemplateID     string          `json:"template_id"`
		SenderDomainID string          `json:"sender_domain_id"`
		MessageType    string          `json:"message_type"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	audienceRef, err := mapAudienceRef(req.AudienceRef)
	if err != nil {
		writeCampaignErr(w, r, domain.ErrPayloadInvalid)
		return
	}

	result, err := h.svc.CreateCampaign(r.Context(), campaignapp.CreateCampaignInput{
		WorkspaceID:    workspaceID,
		UserID:         userID,
		Name:           req.Name,
		AudienceRef:    audienceRef,
		TemplateID:     req.TemplateID,
		SenderDomainID: req.SenderDomainID,
		MessageType:    domain.MessageType(req.MessageType),
		Now:            time.Now().UTC(),
	})
	if err != nil {
		writeCampaignErr(w, r, err)
		return
	}

	writeEnvelope(w, r, http.StatusCreated, campaignResponse(result.Campaign, result.CandidateCount))
}

func (h *campaignHTTP) getCampaign(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	campaignID := chi.URLParam(r, "campaign_id")
	userID, _ := r.Context().Value(ctxUserID).(string)

	result, err := h.svc.GetCampaign(r.Context(), workspaceID, campaignID, userID)
	if err != nil {
		writeCampaignErr(w, r, err)
		return
	}

	writeEnvelope(w, r, http.StatusOK, campaignResponse(result.Campaign, result.CandidateCount))
}

func (h *campaignHTTP) updateCampaign(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	campaignID := chi.URLParam(r, "campaign_id")
	userID, _ := r.Context().Value(ctxUserID).(string)

	var req struct {
		Name           *string         `json:"name"`
		AudienceRef    *audienceRefDTO `json:"audience_ref"`
		TemplateID     *string         `json:"template_id"`
		SenderDomainID *string         `json:"sender_domain_id"`
		MessageType    *string         `json:"message_type"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	input := campaignapp.UpdateCampaignDraftInput{
		WorkspaceID:    workspaceID,
		CampaignID:     campaignID,
		UserID:         userID,
		Name:           req.Name,
		TemplateID:     req.TemplateID,
		SenderDomainID: req.SenderDomainID,
		Now:            time.Now().UTC(),
	}
	if req.MessageType != nil {
		mt := domain.MessageType(*req.MessageType)
		input.MessageType = &mt
	}
	if req.AudienceRef != nil {
		ref, err := mapAudienceRef(req.AudienceRef)
		if err != nil {
			writeCampaignErr(w, r, domain.ErrPayloadInvalid)
			return
		}
		input.AudienceRef = &ref
	}

	result, err := h.svc.UpdateCampaignDraft(r.Context(), input)
	if err != nil {
		writeCampaignErr(w, r, err)
		return
	}

	writeEnvelope(w, r, http.StatusOK, campaignResponse(result.Campaign, result.CandidateCount))
}

func (h *campaignHTTP) scheduleCampaign(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	campaignID := chi.URLParam(r, "campaign_id")
	userID, _ := r.Context().Value(ctxUserID).(string)

	var req struct {
		ScheduledAt *time.Time `json:"scheduled_at"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	result, err := h.svc.ScheduleCampaign(r.Context(), campaignapp.ScheduleCampaignInput{
		WorkspaceID: workspaceID,
		CampaignID:  campaignID,
		UserID:      userID,
		ScheduledAt: req.ScheduledAt,
		Now:         time.Now().UTC(),
	})
	if err != nil {
		writeCampaignErr(w, r, err)
		return
	}

	writeEnvelope(w, r, http.StatusOK, campaignResponse(result.Campaign, result.CandidateCount))
}

func (h *campaignHTTP) cancelCampaign(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	campaignID := chi.URLParam(r, "campaign_id")
	userID, _ := r.Context().Value(ctxUserID).(string)

	result, err := h.svc.CancelCampaign(r.Context(), workspaceID, campaignID, userID, time.Now().UTC())
	if err != nil {
		writeCampaignErr(w, r, err)
		return
	}

	writeEnvelope(w, r, http.StatusOK, campaignResponse(result.Campaign, result.CandidateCount))
}

func (h *campaignHTTP) pauseCampaign(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	campaignID := chi.URLParam(r, "campaign_id")
	userID, _ := r.Context().Value(ctxUserID).(string)

	result, err := h.svc.PauseCampaign(r.Context(), workspaceID, campaignID, userID, time.Now().UTC())
	if err != nil {
		writeCampaignErr(w, r, err)
		return
	}

	writeEnvelope(w, r, http.StatusOK, campaignResponse(result.Campaign, result.CandidateCount))
}

func (h *campaignHTTP) resumeCampaign(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	campaignID := chi.URLParam(r, "campaign_id")
	userID, _ := r.Context().Value(ctxUserID).(string)

	result, err := h.svc.ResumeCampaign(r.Context(), workspaceID, campaignID, userID, time.Now().UTC())
	if err != nil {
		writeCampaignErr(w, r, err)
		return
	}

	writeEnvelope(w, r, http.StatusOK, campaignResponse(result.Campaign, result.CandidateCount))
}

func (h *campaignHTTP) listCampaignCandidates(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	campaignID := chi.URLParam(r, "campaign_id")
	userID, _ := r.Context().Value(ctxUserID).(string)

	results, err := h.svc.ListCampaignCandidates(r.Context(), campaignapp.CandidateListInput{
		WorkspaceID: workspaceID,
		CampaignID:  campaignID,
		Status:      r.URL.Query().Get("status"),
		Cursor:      r.URL.Query().Get("cursor"),
	}, userID)
	if err != nil {
		writeCampaignErr(w, r, err)
		return
	}

	out := make([]map[string]any, 0, len(results.Candidates))
	for _, c := range results.Candidates {
		out = append(out, candidateResponse(c))
	}
	writeEnvelope(w, r, http.StatusOK, map[string]any{
		"candidates":  out,
		"next_cursor": results.NextCursor,
	})
}

type audienceRefDTO struct {
	Type       string   `json:"type"`
	ID         string   `json:"id,omitempty"`
	ContactIDs []string `json:"contact_ids,omitempty"`
}

func mapAudienceRef(dto *audienceRefDTO) (domain.AudienceRef, error) {
	if dto == nil {
		return domain.AudienceRef{}, domain.ErrPayloadInvalid
	}
	ref := domain.AudienceRef{
		Type: domain.AudienceType(dto.Type),
		ID:   dto.ID,
	}
	if dto.Type == string(domain.AudienceTypeContacts) {
		if len(dto.ContactIDs) == 0 {
			return domain.AudienceRef{}, domain.ErrAudienceNotReady
		}
		ref.ContactIDs = dto.ContactIDs
	}
	if !domain.ValidAudienceType(dto.Type) {
		return domain.AudienceRef{}, domain.ErrPayloadInvalid
	}
	return ref, nil
}

func campaignResponse(c domain.Campaign, candidateCount int64) map[string]any {
	out := map[string]any{
		"id":                  c.ID,
		"workspace_id":        c.WorkspaceID,
		"name":                c.Name,
		"status":              string(c.Status),
		"audience_ref":        audienceRefResponse(c.AudienceRef),
		"template_id":         c.TemplateRef.TemplateID,
		"template_version_id": c.TemplateRef.TemplateVersionID,
		"sender_domain_id":    c.SenderDomainID,
		"message_type":        string(c.MessageType),
		"scheduled_at":        c.ScheduledAt,
		"planned_recipients":  c.PlannedRecipients,
		"created_at":          c.CreatedAt,
		"updated_at":          c.UpdatedAt,
		"cancelled_at":        c.CancelledAt,
		"paused_at":           c.PausedAt,
		"completed_at":        c.CompletedAt,
		"summary": map[string]any{
			"planned_recipients": c.PlannedRecipients,
			"queued":             0,
			"delivered":          0,
			"last_updated_at":    c.UpdatedAt,
		},
	}
	if candidateCount > 0 {
		out["candidate_count"] = candidateCount
	}
	return out
}

func audienceRefResponse(ref domain.AudienceRef) map[string]any {
	out := map[string]any{
		"type": string(ref.Type),
	}
	switch ref.Type {
	case domain.AudienceTypeContacts:
		out["contact_ids"] = ref.ContactIDs
	case domain.AudienceTypeList:
		out["id"] = ref.ID
	case domain.AudienceTypeSegment:
		out["id"] = ref.ID
	}
	return out
}

func candidateResponse(c domain.CampaignMessageCandidate) map[string]any {
	return map[string]any{
		"id":               c.ID,
		"contact_id":       c.ContactID,
		"email_normalized": c.EmailNormalized,
		"status":           string(c.Status),
		"created_at":       c.CreatedAt,
		"updated_at":       c.UpdatedAt,
	}
}

func writeCampaignErr(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrReadDenied):
		writeError(w, r, http.StatusForbidden, "campaign.read_denied", err.Error(), nil)
	case errors.Is(err, domain.ErrWriteDenied):
		writeError(w, r, http.StatusForbidden, "campaign.write_denied", err.Error(), nil)
	case errors.Is(err, domain.ErrSendDenied):
		writeError(w, r, http.StatusForbidden, "campaign.send_denied", err.Error(), nil)
	case errors.Is(err, domain.ErrCampaignNotFound):
		writeError(w, r, http.StatusNotFound, "campaign.not_found", err.Error(), nil)
	case errors.Is(err, domain.ErrPayloadInvalid):
		writeError(w, r, http.StatusUnprocessableEntity, "campaign.payload_invalid", err.Error(), nil)
	case errors.Is(err, domain.ErrAudienceNotReady):
		writeError(w, r, http.StatusUnprocessableEntity, "campaign.audience_not_ready", err.Error(), nil)
	case errors.Is(err, domain.ErrSenderNotVerified):
		writeError(w, r, http.StatusUnprocessableEntity, "sender.domain_not_verified", err.Error(), nil)
	case errors.Is(err, domain.ErrTemplatePublishRequired):
		writeError(w, r, http.StatusUnprocessableEntity, "template.publish_required", err.Error(), nil)
	case errors.Is(err, domain.ErrInvalidStateTransition):
		writeError(w, r, http.StatusConflict, "campaign.invalid_state_transition", err.Error(), nil)
	case errors.Is(err, auth.ErrPermissionDenied):
		writeError(w, r, http.StatusForbidden, "auth.permission_denied", err.Error(), nil)
	default:
		writeError(w, r, http.StatusInternalServerError, "internal.error", "internal error", nil)
	}
}
