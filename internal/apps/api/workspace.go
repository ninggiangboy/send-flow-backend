package api

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	identityapp "github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
)

type workspaceHTTP struct {
	svc           *identityapp.Service
	auditRecorder identityapp.AuditRecorder
}

func newWorkspaceHTTP(svc *identityapp.Service, auditRecorder identityapp.AuditRecorder) *workspaceHTTP {
	return &workspaceHTTP{svc: svc, auditRecorder: auditRecorder}
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
	out := make([]workspaceResponseData, 0, len(workspaces))
	for _, ws := range workspaces {
		out = append(out, workspaceResponse(&ws, ""))
	}
	writeEnvelope(w, r, http.StatusOK, out)
}

func (h *workspaceHTTP) getWorkspace(w http.ResponseWriter, r *http.Request) {
	workspaceID := workspaceIDParam(r)
	userID, _ := r.Context().Value(ctxUserID).(string)
	ws, err := h.svc.GetWorkspace(r.Context(), workspaceID, userID)
	if err != nil {
		writeWorkspaceErr(w, r, err)
		return
	}
	writeEnvelope(w, r, http.StatusOK, workspaceResponse(ws, ""))
}

func (h *workspaceHTTP) getWorkspaceAccess(w http.ResponseWriter, r *http.Request) {
	workspaceID := workspaceIDParam(r)
	userID, _ := r.Context().Value(ctxUserID).(string)
	membership, err := h.svc.GetWorkspaceAccess(r.Context(), workspaceID, userID)
	if err != nil {
		writeWorkspaceErr(w, r, err)
		return
	}
	writeEnvelope(w, r, http.StatusOK, WorkspaceAccessResponse{
		WorkspaceID:          membership.WorkspaceID,
		MembershipID:         membership.ID,
		Status:               string(membership.Status),
		RoleIDs:              membership.RoleIDs,
		RoleNames:            membership.RoleNames,
		EffectivePermissions: membership.EffectivePermissions,
	})
}

func (h *workspaceHTTP) listWorkspaceMembers(w http.ResponseWriter, r *http.Request) {
	workspaceID := workspaceIDParam(r)
	userID, _ := r.Context().Value(ctxUserID).(string)
	members, err := h.svc.ListWorkspaceMembers(r.Context(), workspaceID, userID)
	if err != nil {
		writeWorkspaceErr(w, r, err)
		return
	}
	out := make([]membershipResponseData, 0, len(members))
	for _, m := range members {
		out = append(out, membershipResponse(m))
	}
	writeEnvelope(w, r, http.StatusOK, out)
}

func (h *workspaceHTTP) listWorkspaceInvitations(w http.ResponseWriter, r *http.Request) {
	workspaceID := workspaceIDParam(r)
	userID, _ := r.Context().Value(ctxUserID).(string)
	invitations, err := h.svc.ListWorkspaceInvitations(r.Context(), workspaceID, userID)
	if err != nil {
		writeWorkspaceErr(w, r, err)
		return
	}
	out := make([]invitationResponseData, 0, len(invitations))
	for _, inv := range invitations {
		out = append(out, invitationResponse(inv))
	}
	writeEnvelope(w, r, http.StatusOK, out)
}

func (h *workspaceHTTP) inviteWorkspaceMember(w http.ResponseWriter, r *http.Request) {
	workspaceID := workspaceIDParam(r)
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
	invID := ""
	if result.Invitation != nil {
		invID = result.Invitation.ID
	}
	h.recordAudit(r, identityapp.RecordAuditInput{
		WorkspaceID: workspaceID,
		ActorUserID: userID,
		ActionType:  auditActionWorkspaceInvitationCreated,
		TargetType:  "invitation",
		TargetID:    invID,
		PayloadSummary: map[string]any{
			"email":    req.Email,
			"role_ids": req.RoleIDs,
		},
	})
	if result.Invitation != nil {
		writeEnvelope(w, r, http.StatusOK, invitationResponse(*result.Invitation))
	} else {
		writeEnvelope(w, r, http.StatusOK, statusResponseDoc{Status: "invited"})
	}
}

func (h *workspaceHTTP) acceptWorkspaceInvitation(w http.ResponseWriter, r *http.Request) {
	token := tokenParam(r)
	userID, _ := r.Context().Value(ctxUserID).(string)
	membership, err := h.svc.AcceptWorkspaceInvitation(r.Context(), token, userID, time.Now().UTC())
	if err != nil {
		writeWorkspaceErr(w, r, err)
		return
	}
	h.recordAudit(r, identityapp.RecordAuditInput{
		WorkspaceID: membership.WorkspaceID,
		ActorUserID: userID,
		ActionType:  auditActionWorkspaceInvitationAccepted,
		TargetType:  "membership",
		TargetID:    membership.ID,
	})
	writeEnvelope(w, r, http.StatusOK, membershipResponse(*membership))
}

func (h *workspaceHTTP) removeWorkspaceMember(w http.ResponseWriter, r *http.Request) {
	workspaceID := workspaceIDParam(r)
	membershipID := pathParam(r, "membership_id")
	userID, _ := r.Context().Value(ctxUserID).(string)
	if err := h.svc.RemoveWorkspaceMember(r.Context(), workspaceID, membershipID, userID, time.Now().UTC()); err != nil {
		writeWorkspaceErr(w, r, err)
		return
	}
	h.recordAudit(r, identityapp.RecordAuditInput{
		WorkspaceID:    workspaceID,
		ActorUserID:    userID,
		ActionType:     auditActionWorkspaceMemberRemoved,
		TargetType:     "membership",
		TargetID:       membershipID,
		PayloadSummary: map[string]any{},
	})
	w.WriteHeader(http.StatusNoContent)
}

func (h *workspaceHTTP) updateWorkspaceMemberRole(w http.ResponseWriter, r *http.Request) {
	workspaceID := workspaceIDParam(r)
	membershipID := pathParam(r, "membership_id")
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
	h.recordAudit(r, identityapp.RecordAuditInput{
		WorkspaceID: workspaceID,
		ActorUserID: userID,
		ActionType:  auditActionWorkspaceMemberRoleUpdated,
		TargetType:  "membership",
		TargetID:    membershipID,
		PayloadSummary: map[string]any{
			"role_ids": req.RoleIDs,
		},
	})
	writeEnvelope(w, r, http.StatusOK, statusResponseDoc{Updated: ptrBool(true)})
}

func (h *workspaceHTTP) assignWorkspaceMemberRoles(w http.ResponseWriter, r *http.Request) {
	workspaceID := workspaceIDParam(r)
	membershipID := pathParam(r, "membership_id")
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
	h.recordAudit(r, identityapp.RecordAuditInput{
		WorkspaceID: workspaceID,
		ActorUserID: userID,
		ActionType:  auditActionWorkspaceMemberRolesAssigned,
		TargetType:  "membership",
		TargetID:    membershipID,
		PayloadSummary: map[string]any{
			"role_ids": req.RoleIDs,
		},
	})
	writeEnvelope(w, r, http.StatusOK, membershipRoleAssignmentResponse(*membership))
}

func (h *workspaceHTTP) listWorkspaceRoles(w http.ResponseWriter, r *http.Request) {
	workspaceID := workspaceIDParam(r)
	userID, _ := r.Context().Value(ctxUserID).(string)
	roles, err := h.svc.ListWorkspaceRoles(r.Context(), workspaceID, userID)
	if err != nil {
		writeWorkspaceErr(w, r, err)
		return
	}
	out := make([]roleResponseData, 0, len(roles))
	for _, role := range roles {
		out = append(out, roleResponse(role))
	}
	writeEnvelope(w, r, http.StatusOK, out)
}

func (h *workspaceHTTP) createWorkspaceRole(w http.ResponseWriter, r *http.Request) {
	workspaceID := workspaceIDParam(r)
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
	h.recordAudit(r, identityapp.RecordAuditInput{
		WorkspaceID: workspaceID,
		ActorUserID: userID,
		ActionType:  auditActionWorkspaceRoleCreated,
		TargetType:  "role",
		TargetID:    role.ID,
		PayloadSummary: map[string]any{
			"name":             req.Name,
			"permission_names": req.PermissionNames,
		},
	})
	writeEnvelope(w, r, http.StatusCreated, roleResponse(*role))
}

func (h *workspaceHTTP) updateWorkspaceRole(w http.ResponseWriter, r *http.Request) {
	workspaceID := workspaceIDParam(r)
	roleID := pathParam(r, "role_id")
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
	h.recordAudit(r, identityapp.RecordAuditInput{
		WorkspaceID: workspaceID,
		ActorUserID: userID,
		ActionType:  auditActionWorkspaceRoleUpdated,
		TargetType:  "role",
		TargetID:    roleID,
		PayloadSummary: map[string]any{
			"name":             req.Name,
			"permission_names": req.PermissionNames,
			"status":           req.Status,
		},
	})
	writeEnvelope(w, r, http.StatusOK, roleResponse(*role))
}

func (h *workspaceHTTP) listPermissions(w http.ResponseWriter, r *http.Request) {
	userID, _ := r.Context().Value(ctxUserID).(string)
	permissions, err := h.svc.ListPermissions(r.Context(), userID)
	if err != nil {
		writeWorkspaceErr(w, r, err)
		return
	}
	out := make([]PermissionResponse, 0, len(permissions))
	for _, permission := range permissions {
		out = append(out, PermissionResponse{Bit: int(permission.Bit), Name: permission.Name})
	}
	writeEnvelope(w, r, http.StatusOK, out)
}

func writeWorkspaceErr(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrWorkspaceNotFound):
		writeError(w, r, http.StatusNotFound, errCodeIdentityWorkspaceNotFound, err.Error(), nil)
	case errors.Is(err, domain.ErrWorkspaceAccessDenied):
		writeError(w, r, http.StatusForbidden, errCodeIdentityWorkspaceAccessDenied, err.Error(), nil)
	case errors.Is(err, domain.ErrMembershipNotFound):
		writeError(w, r, http.StatusNotFound, errCodeIdentityMembershipNotFound, err.Error(), nil)
	case errors.Is(err, domain.ErrMembershipManageDenied):
		writeError(w, r, http.StatusForbidden, errCodeIdentityMembershipManageDenied, err.Error(), nil)
	case errors.Is(err, domain.ErrInvitationNotFound):
		writeError(w, r, http.StatusNotFound, errCodeIdentityInvitationNotFound, err.Error(), nil)
	case errors.Is(err, domain.ErrInvitationExpired):
		writeError(w, r, http.StatusGone, errCodeIdentityInvitationTokenExpired, err.Error(), nil)
	case errors.Is(err, domain.ErrInvitationAccepted):
		writeError(w, r, http.StatusConflict, errCodeIdentityInvitationAlreadyAccepted, err.Error(), nil)
	case errors.Is(err, domain.ErrLastOwnerCannotBeRemoved):
		writeError(w, r, http.StatusConflict, errCodeIdentityLastOwnerCannotBeRemoved, err.Error(), nil)
	case errors.Is(err, domain.ErrInvitationPayloadInvalid):
		writeError(w, r, http.StatusUnprocessableEntity, errCodeIdentityInvitationPayloadInvalid, err.Error(), nil)
	case errors.Is(err, domain.ErrInvalidRole):
		writeError(w, r, http.StatusUnprocessableEntity, errCodeIdentityInvitationPayloadInvalid, err.Error(), nil)
	case errors.Is(err, domain.ErrRoleNotFound):
		writeError(w, r, http.StatusNotFound, errCodeIdentityRoleNotFound, err.Error(), nil)
	case errors.Is(err, domain.ErrRoleManageDenied):
		writeError(w, r, http.StatusForbidden, errCodeIdentityRoleManageDenied, err.Error(), nil)
	case errors.Is(err, domain.ErrRoleNameConflict):
		writeError(w, r, http.StatusConflict, errCodeIdentityRoleNameConflict, err.Error(), nil)
	case errors.Is(err, domain.ErrPermissionSetInvalid):
		writeError(w, r, http.StatusUnprocessableEntity, errCodeIdentityPermissionSetInvalid, err.Error(), nil)
	case errors.Is(err, domain.ErrPermissionRegistryUnknown):
		writeError(w, r, http.StatusUnprocessableEntity, errCodeIdentityPermissionRegistryUnknown, err.Error(), nil)
	case errors.Is(err, domain.ErrRoleAssignmentConflict):
		writeError(w, r, http.StatusConflict, errCodeIdentityRoleAssignmentConflict, err.Error(), nil)
	case errors.Is(err, domain.ErrInvalidPermissionMask):
		writeError(w, r, http.StatusUnprocessableEntity, errCodeIdentityInvalidPermissionMask, err.Error(), nil)
	case errors.Is(err, domain.ErrInvalidWorkspaceName):
		writeError(w, r, http.StatusUnprocessableEntity, errCodeIdentityInvalidWorkspaceName, err.Error(), nil)
	case errors.Is(err, domain.ErrWorkspaceNameConflict):
		writeError(w, r, http.StatusConflict, errCodeIdentityWorkspaceNameConflict, err.Error(), nil)
	default:
		writeInternalError(w, r)
	}
}

type workspaceResponseData struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	MembershipID string    `json:"membership_id"`
	RoleNames    []string  `json:"role_names"`
	Plan         *string   `json:"plan,omitempty"`
	LogoIcon     *string   `json:"logo_icon,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func workspaceResponse(ws *domain.Workspace, role string) workspaceResponseData {
	out := workspaceResponseData{
		ID:           ws.ID,
		Name:         ws.Name,
		MembershipID: ws.MembershipID,
		RoleNames:    ws.RoleNames,
		CreatedAt:    ws.CreatedAt,
		UpdatedAt:    ws.UpdatedAt,
	}
	if ws.Plan != nil {
		out.Plan = ws.Plan
	}
	if ws.LogoIcon != nil {
		out.LogoIcon = ws.LogoIcon
	}
	return out
}

type membershipResponseData struct {
	MembershipID string   `json:"membership_id"`
	WorkspaceID  string   `json:"workspace_id"`
	UserEmail    string   `json:"user_email"`
	Status       string   `json:"status"`
	RoleNames    []string `json:"role_names"`
}

func membershipResponse(m domain.Membership) membershipResponseData {
	userEmail := ""
	if m.UserEmail != nil {
		userEmail = *m.UserEmail
	}
	return membershipResponseData{
		MembershipID: m.ID,
		WorkspaceID:  m.WorkspaceID,
		UserEmail:    userEmail,
		Status:       string(m.Status),
		RoleNames:    m.RoleNames,
	}
}

type invitationResponseData struct {
	Token       string     `json:"token"`
	WorkspaceID string     `json:"workspace_id"`
	Email       string     `json:"email"`
	RoleIDs     []string   `json:"role_ids"`
	Status      string     `json:"status"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

func invitationResponse(inv domain.Invitation) invitationResponseData {
	var expiresAt *time.Time
	if !inv.ExpiresAt.IsZero() {
		expiresAt = &inv.ExpiresAt
	}
	return invitationResponseData{
		Token:       inv.Token,
		WorkspaceID: inv.WorkspaceID,
		Email:       inv.Email,
		RoleIDs:     inv.RoleIDs,
		Status:      string(inv.Status),
		ExpiresAt:   expiresAt,
		CreatedAt:   inv.CreatedAt,
	}
}

type membershipRoleAssignmentResponseData struct {
	WorkspaceID          string   `json:"workspace_id"`
	MembershipID         string   `json:"membership_id"`
	RoleIDs              []string `json:"role_ids"`
	EffectivePermissions []string `json:"effective_permissions"`
}

func membershipRoleAssignmentResponse(m domain.Membership) membershipRoleAssignmentResponseData {
	return membershipRoleAssignmentResponseData{
		WorkspaceID:          m.WorkspaceID,
		MembershipID:         m.ID,
		RoleIDs:              m.RoleIDs,
		EffectivePermissions: m.EffectivePermissions,
	}
}

type roleResponseData struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Type            string   `json:"type"`
	PermissionsMask int64    `json:"permissions_mask"`
	PermissionNames []string `json:"permission_names"`
	Builtin         bool     `json:"builtin"`
	Status          string   `json:"status"`
	Version         int      `json:"version"`
}

func roleResponse(role domain.Role) roleResponseData {
	return roleResponseData{
		ID:              role.ID,
		Name:            role.Name,
		Type:            string(role.Type),
		PermissionsMask: role.PermissionsMask,
		PermissionNames: role.PermissionNames(),
		Builtin:         role.Builtin,
		Status:          string(role.Status),
		Version:         role.Version,
	}
}

func (h *workspaceHTTP) recordAudit(r *http.Request, input identityapp.RecordAuditInput) {
	if h.auditRecorder == nil {
		return
	}
	reqCtx := r.Context().Value(ctxRequestContext).(*requestLogContext)
	input.RequestID = reqCtx.RequestID
	input.OccurredAt = time.Now().UTC()
	if err := h.auditRecorder.Record(r.Context(), input); err != nil {
		slog.Warn("failed to record audit event", "error", err)
	}
}
