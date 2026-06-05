package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	identityapp "github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
)

type workspaceHTTP struct {
	svc *identityapp.Service
}

func newWorkspaceHTTP(svc *identityapp.Service) *workspaceHTTP {
	return &workspaceHTTP{svc: svc}
}

func (h *workspaceHTTP) registerRoutes(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(authzMiddleware(h.svc))
		r.Post("/workspaces", h.createWorkspace)
		r.Get("/workspaces", h.listWorkspaces)
		r.Get("/permissions", h.listPermissions)

		r.Route("/workspaces/{workspace_id}", func(r chi.Router) {
			r.Get("/", h.getWorkspace)
			r.Get("/access", h.getWorkspaceAccess)
			r.Get("/members", h.listWorkspaceMembers)
			r.Get("/invitations", h.listWorkspaceInvitations)
			r.Get("/roles", h.listWorkspaceRoles)
			r.Post("/roles", h.createWorkspaceRole)
			r.Post("/invitations", h.inviteWorkspaceMember)
			r.Delete("/members/{membership_id}", h.removeWorkspaceMember)
			r.Put("/members/{membership_id}/role", h.updateWorkspaceMemberRole)
			r.Put("/members/{membership_id}/roles", h.assignWorkspaceMemberRoles)
			r.Patch("/roles/{role_id}", h.updateWorkspaceRole)
		})

		r.Post("/invitations/{token}/accept", h.acceptWorkspaceInvitation)
	})
}

func (h *workspaceHTTP) createWorkspace(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	userID, _ := r.Context().Value(ctxUserID).(string)
	ws, err := h.svc.CreateWorkspace(r.Context(), req.Name, userID, time.Now().UTC())
	if err != nil {
		writeWorkspaceErr(w, r, err)
		return
	}
	writeEnvelope(w, r, http.StatusCreated, workspaceResponse(ws, string(domain.MembershipRoleOwner)))
}

func (h *workspaceHTTP) listWorkspaces(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(ctxUserID).(string)
	workspaces, err := h.svc.ListWorkspaces(r.Context(), userID)
	if err != nil {
		writeWorkspaceErr(w, r, err)
		return
	}
	out := make([]map[string]any, 0, len(workspaces))
	for _, ws := range workspaces {
		out = append(out, workspaceResponse(&ws, ""))
	}
	writeEnvelope(w, r, http.StatusOK, out)
}

func (h *workspaceHTTP) getWorkspace(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	userID, _ := r.Context().Value(ctxUserID).(string)
	ws, err := h.svc.GetWorkspace(r.Context(), workspaceID, userID)
	if err != nil {
		writeWorkspaceErr(w, r, err)
		return
	}
	writeEnvelope(w, r, http.StatusOK, workspaceResponse(ws, ""))
}

func (h *workspaceHTTP) getWorkspaceAccess(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	userID, _ := r.Context().Value(ctxUserID).(string)
	membership, err := h.svc.GetWorkspaceAccess(r.Context(), workspaceID, userID)
	if err != nil {
		writeWorkspaceErr(w, r, err)
		return
	}
	writeEnvelope(w, r, http.StatusOK, map[string]any{
		"workspace_id":          membership.WorkspaceID,
		"membership_id":         membership.ID,
		"status":                string(membership.Status),
		"role_ids":              membership.RoleIDs,
		"role_names":            membership.RoleNames,
		"effective_permissions": membership.EffectivePermissions,
	})
}

func (h *workspaceHTTP) listWorkspaceMembers(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	userID, _ := r.Context().Value(ctxUserID).(string)
	members, err := h.svc.ListWorkspaceMembers(r.Context(), workspaceID, userID)
	if err != nil {
		writeWorkspaceErr(w, r, err)
		return
	}
	out := make([]map[string]any, 0, len(members))
	for _, m := range members {
		out = append(out, membershipResponse(m))
	}
	writeEnvelope(w, r, http.StatusOK, out)
}

func (h *workspaceHTTP) listWorkspaceInvitations(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	userID, _ := r.Context().Value(ctxUserID).(string)
	invitations, err := h.svc.ListWorkspaceInvitations(r.Context(), workspaceID, userID)
	if err != nil {
		writeWorkspaceErr(w, r, err)
		return
	}
	out := make([]map[string]any, 0, len(invitations))
	for _, inv := range invitations {
		out = append(out, invitationResponse(inv))
	}
	writeEnvelope(w, r, http.StatusOK, out)
}

func (h *workspaceHTTP) inviteWorkspaceMember(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	var req struct {
		Email   string   `json:"email"`
		RoleIDs []string `json:"role_ids"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	userID, _ := r.Context().Value(ctxUserID).(string)
	result, err := h.svc.InviteWorkspaceMember(r.Context(), workspaceID, req.Email, req.RoleIDs, userID, time.Now().UTC())
	if err != nil {
		writeWorkspaceErr(w, r, err)
		return
	}
	if result.Invitation != nil {
		writeEnvelope(w, r, http.StatusOK, invitationResponse(*result.Invitation))
	} else {
		writeEnvelope(w, r, http.StatusOK, map[string]string{"status": "invited"})
	}
}

func (h *workspaceHTTP) acceptWorkspaceInvitation(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	userID, _ := r.Context().Value(ctxUserID).(string)
	membership, err := h.svc.AcceptWorkspaceInvitation(r.Context(), token, userID, time.Now().UTC())
	if err != nil {
		writeWorkspaceErr(w, r, err)
		return
	}
	writeEnvelope(w, r, http.StatusOK, membershipResponse(*membership))
}

func (h *workspaceHTTP) removeWorkspaceMember(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	membershipID := chi.URLParam(r, "membership_id")
	userID, _ := r.Context().Value(ctxUserID).(string)
	if err := h.svc.RemoveWorkspaceMember(r.Context(), workspaceID, membershipID, userID, time.Now().UTC()); err != nil {
		writeWorkspaceErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *workspaceHTTP) updateWorkspaceMemberRole(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	membershipID := chi.URLParam(r, "membership_id")
	var req struct {
		RoleIDs []string `json:"role_ids"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	userID, _ := r.Context().Value(ctxUserID).(string)
	if err := h.svc.UpdateWorkspaceMemberRole(r.Context(), workspaceID, membershipID, req.RoleIDs, userID, time.Now().UTC()); err != nil {
		writeWorkspaceErr(w, r, err)
		return
	}
	writeEnvelope(w, r, http.StatusOK, map[string]bool{"updated": true})
}

func (h *workspaceHTTP) assignWorkspaceMemberRoles(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	membershipID := chi.URLParam(r, "membership_id")
	var req struct {
		RoleIDs []string `json:"role_ids"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	userID, _ := r.Context().Value(ctxUserID).(string)
	membership, err := h.svc.AssignWorkspaceMemberRoles(r.Context(), workspaceID, membershipID, req.RoleIDs, userID, time.Now().UTC())
	if err != nil {
		writeWorkspaceErr(w, r, err)
		return
	}
	writeEnvelope(w, r, http.StatusOK, membershipRoleAssignmentResponse(*membership))
}

func (h *workspaceHTTP) listWorkspaceRoles(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	userID, _ := r.Context().Value(ctxUserID).(string)
	roles, err := h.svc.ListWorkspaceRoles(r.Context(), workspaceID, userID)
	if err != nil {
		writeWorkspaceErr(w, r, err)
		return
	}
	out := make([]map[string]any, 0, len(roles))
	for _, role := range roles {
		out = append(out, roleResponse(role))
	}
	writeEnvelope(w, r, http.StatusOK, out)
}

func (h *workspaceHTTP) createWorkspaceRole(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	var req struct {
		Name            string   `json:"name"`
		PermissionNames []string `json:"permission_names"`
		PermissionsMask *int64   `json:"permissions_mask"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	userID, _ := r.Context().Value(ctxUserID).(string)
	role, err := h.svc.CreateWorkspaceRole(r.Context(), workspaceID, userID, req.Name, req.PermissionNames, req.PermissionsMask, time.Now().UTC())
	if err != nil {
		writeWorkspaceErr(w, r, err)
		return
	}
	writeEnvelope(w, r, http.StatusCreated, roleResponse(*role))
}

func (h *workspaceHTTP) updateWorkspaceRole(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")
	roleID := chi.URLParam(r, "role_id")
	var req struct {
		Name            *string  `json:"name"`
		PermissionNames []string `json:"permission_names"`
		PermissionsMask *int64   `json:"permissions_mask"`
		Status          *string  `json:"status"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	userID, _ := r.Context().Value(ctxUserID).(string)
	role, err := h.svc.UpdateWorkspaceRole(r.Context(), workspaceID, roleID, userID, req.Name, req.PermissionNames, req.PermissionsMask, req.Status, time.Now().UTC())
	if err != nil {
		writeWorkspaceErr(w, r, err)
		return
	}
	writeEnvelope(w, r, http.StatusOK, roleResponse(*role))
}

func (h *workspaceHTTP) listPermissions(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(ctxUserID).(string)
	permissions, err := h.svc.ListPermissions(r.Context(), userID)
	if err != nil {
		writeWorkspaceErr(w, r, err)
		return
	}
	out := make([]map[string]any, 0, len(permissions))
	for _, permission := range permissions {
		out = append(out, map[string]any{
			"bit":  permission.Bit,
			"name": permission.Name,
		})
	}
	writeEnvelope(w, r, http.StatusOK, out)
}

func writeWorkspaceErr(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrWorkspaceNotFound):
		writeError(w, r, http.StatusNotFound, "identity.workspace_not_found", err.Error(), nil)
	case errors.Is(err, domain.ErrWorkspaceAccessDenied):
		writeError(w, r, http.StatusForbidden, "identity.workspace_access_denied", err.Error(), nil)
	case errors.Is(err, domain.ErrMembershipNotFound):
		writeError(w, r, http.StatusNotFound, "identity.membership_not_found", err.Error(), nil)
	case errors.Is(err, domain.ErrMembershipManageDenied):
		writeError(w, r, http.StatusForbidden, "identity.membership_manage_denied", err.Error(), nil)
	case errors.Is(err, domain.ErrInvitationNotFound):
		writeError(w, r, http.StatusNotFound, "identity.invitation_not_found", err.Error(), nil)
	case errors.Is(err, domain.ErrInvitationExpired):
		writeError(w, r, http.StatusGone, "identity.invitation_token_expired", err.Error(), nil)
	case errors.Is(err, domain.ErrInvitationAccepted):
		writeError(w, r, http.StatusConflict, "identity.invitation_already_accepted", err.Error(), nil)
	case errors.Is(err, domain.ErrLastOwnerCannotBeRemoved):
		writeError(w, r, http.StatusConflict, "identity.last_owner_cannot_be_removed", err.Error(), nil)
	case errors.Is(err, domain.ErrInvitationPayloadInvalid):
		writeError(w, r, http.StatusUnprocessableEntity, "identity.invitation_payload_invalid", err.Error(), nil)
	case errors.Is(err, domain.ErrInvalidRole):
		writeError(w, r, http.StatusUnprocessableEntity, "identity.invitation_payload_invalid", err.Error(), nil)
	case errors.Is(err, domain.ErrRoleNotFound):
		writeError(w, r, http.StatusNotFound, "identity.role_not_found", err.Error(), nil)
	case errors.Is(err, domain.ErrRoleManageDenied):
		writeError(w, r, http.StatusForbidden, "identity.role_manage_denied", err.Error(), nil)
	case errors.Is(err, domain.ErrRoleNameConflict):
		writeError(w, r, http.StatusConflict, "identity.role_name_conflict", err.Error(), nil)
	case errors.Is(err, domain.ErrPermissionSetInvalid):
		writeError(w, r, http.StatusUnprocessableEntity, "identity.permission_set_invalid", err.Error(), nil)
	case errors.Is(err, domain.ErrPermissionRegistryUnknown):
		writeError(w, r, http.StatusUnprocessableEntity, "identity.permission_registry_unknown", err.Error(), nil)
	case errors.Is(err, domain.ErrRoleAssignmentConflict):
		writeError(w, r, http.StatusConflict, "identity.role_assignment_conflict", err.Error(), nil)
	case errors.Is(err, domain.ErrInvalidPermissionMask):
		writeError(w, r, http.StatusUnprocessableEntity, "identity.invalid_permission_mask", err.Error(), nil)
	case errors.Is(err, domain.ErrInvalidWorkspaceName):
		writeError(w, r, http.StatusUnprocessableEntity, "identity.invalid_workspace_name", err.Error(), nil)
	case errors.Is(err, domain.ErrWorkspaceNameConflict):
		writeError(w, r, http.StatusConflict, "identity.workspace_name_conflict", err.Error(), nil)
	default:
		writeError(w, r, http.StatusInternalServerError, "health.runtime_not_ready", "internal error", nil)
	}
}

func workspaceResponse(ws *domain.Workspace, role string) map[string]any {
	out := map[string]any{
		"id":            ws.ID,
		"name":          ws.Name,
		"membership_id": ws.MembershipID,
		"role_names":    ws.RoleNames,
		"created_at":    ws.CreatedAt,
		"updated_at":    ws.UpdatedAt,
	}
	if ws.Plan != nil {
		out["plan"] = *ws.Plan
	}
	if ws.LogoIcon != nil {
		out["logo_icon"] = *ws.LogoIcon
	}
	return out
}

func membershipResponse(m domain.Membership) map[string]any {
	return map[string]any{
		"membership_id": m.ID,
		"workspace_id":  m.WorkspaceID,
		"user_email":    m.UserEmail,
		"status":        string(m.Status),
		"role_names":    m.RoleNames,
	}
}

func invitationResponse(inv domain.Invitation) map[string]any {
	expiresAt := any(nil)
	if !inv.ExpiresAt.IsZero() {
		expiresAt = inv.ExpiresAt
	}
	return map[string]any{
		"token":        inv.Token,
		"workspace_id": inv.WorkspaceID,
		"email":        inv.Email,
		"role_ids":     inv.RoleIDs,
		"status":       string(inv.Status),
		"expires_at":   expiresAt,
		"created_at":   inv.CreatedAt,
	}
}

func membershipRoleAssignmentResponse(m domain.Membership) map[string]any {
	return map[string]any{
		"workspace_id":          m.WorkspaceID,
		"membership_id":         m.ID,
		"role_ids":              m.RoleIDs,
		"effective_permissions": m.EffectivePermissions,
	}
}

func roleResponse(role domain.Role) map[string]any {
	return map[string]any{
		"id":               role.ID,
		"name":             role.Name,
		"type":             string(role.Type),
		"permissions_mask": role.PermissionsMask,
		"permission_names": role.PermissionNames(),
		"builtin":          role.Builtin,
		"status":           string(role.Status),
		"version":          role.Version,
	}
}
