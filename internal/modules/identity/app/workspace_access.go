package app

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/inviteworkspacemember"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/updateworkspacememberrole"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
)

// ListWorkspaces returns all workspaces the user belongs to, enriched with the user's
// membership ID and role names for each workspace.
func (s *Service) ListWorkspaces(ctx context.Context, userID string) ([]domain.Workspace, error) {
	workspaces, err := s.queries.ListWorkspaces(ctx, userID)
	if err != nil {
		return nil, err
	}
	for i := range workspaces {
		membership, err := s.enrichedMembershipForUser(ctx, workspaces[i].ID, userID)
		if err != nil {
			return nil, err
		}
		workspaces[i].MembershipID = membership.ID
		workspaces[i].RoleNames = membership.RoleNames
	}
	return workspaces, nil
}

// GetWorkspace returns a workspace by ID if the user is a member, enriched with membership
// metadata (membership ID and role names).
func (s *Service) GetWorkspace(ctx context.Context, workspaceID, userID string) (*domain.Workspace, error) {
	workspace, err := s.queries.GetWorkspace(ctx, workspaceID, userID)
	if err != nil {
		return nil, err
	}
	membership, err := s.enrichedMembershipForUser(ctx, workspaceID, userID)
	if err != nil {
		return nil, err
	}
	workspace.MembershipID = membership.ID
	workspace.RoleNames = membership.RoleNames
	return workspace, nil
}

// ListWorkspaceMembers returns all memberships in a workspace, each enriched with role IDs,
// role names, and effective permissions. Requires the workspace:read permission.
func (s *Service) ListWorkspaceMembers(ctx context.Context, workspaceID, userID string) ([]domain.Membership, error) {
	if _, err := s.requireWorkspacePermission(ctx, workspaceID, userID, domain.PermissionWorkspaceRead); err != nil {
		return nil, err
	}
	memberships, err := s.queries.ListWorkspaceMembers(ctx, workspaceID, userID)
	if err != nil {
		return nil, err
	}
	for i := range memberships {
		if err := s.enrichMembership(ctx, &memberships[i]); err != nil {
			return nil, err
		}
	}
	return memberships, nil
}

// GetWorkspaceAccess returns the user's membership for a workspace, fully enriched with
// role IDs, role names, and effective permissions. Used by clients to determine what the
// current user can do in a workspace.
func (s *Service) GetWorkspaceAccess(ctx context.Context, workspaceID, userID string) (*domain.Membership, error) {
	return s.enrichedMembershipForUser(ctx, workspaceID, userID)
}

// ListWorkspaceInvitations returns all invitations for a workspace. Requires the
// workspace:manage_members permission. Each invitation is enriched with role IDs and
// the derived legacy role.
func (s *Service) ListWorkspaceInvitations(ctx context.Context, workspaceID, userID string) ([]domain.Invitation, error) {
	if _, err := s.requireWorkspacePermission(ctx, workspaceID, userID, domain.PermissionWorkspaceManageMembers); err != nil {
		return nil, err
	}
	invitations, err := s.queries.ListWorkspaceInvitations(ctx, workspaceID, userID)
	if err != nil {
		return nil, err
	}
	for i := range invitations {
		if err := s.enrichInvitation(ctx, &invitations[i]); err != nil {
			return nil, err
		}
	}
	return invitations, nil
}

// InviteWorkspaceMember delegates to the invite workspace member command handler.
func (s *Service) InviteWorkspaceMember(ctx context.Context, workspaceID, email string, roleIDs []string, inviterID string, now time.Time) (*inviteworkspacemember.Result, error) {
	return s.commands.InviteWorkspaceMember(ctx, inviteworkspacemember.Command{WorkspaceID: workspaceID, Email: email, RoleIDs: roleIDs, InviterID: inviterID, Now: now})
}

// UpdateWorkspaceMemberRole delegates to the update workspace member role command handler.
func (s *Service) UpdateWorkspaceMemberRole(ctx context.Context, workspaceID, membershipID string, roleIDs []string, updaterID string, now time.Time) error {
	return s.commands.UpdateWorkspaceMemberRole(ctx, updateworkspacememberrole.Command{WorkspaceID: workspaceID, MembershipID: membershipID, RoleIDs: roleIDs, UpdaterID: updaterID, Now: now})
}

// AssignWorkspaceMemberRoles updates a member's roles and returns the enriched membership.
// It combines the UpdateWorkspaceMemberRole command with a read-back to return the updated
// membership with role and permission metadata.
func (s *Service) AssignWorkspaceMemberRoles(ctx context.Context, workspaceID, membershipID string, roleIDs []string, updaterID string, now time.Time) (*domain.Membership, error) {
	if err := s.UpdateWorkspaceMemberRole(ctx, workspaceID, membershipID, roleIDs, updaterID, now); err != nil {
		return nil, err
	}
	membership, err := s.deps.MembershipsRead.FindByID(ctx, membershipID)
	if err != nil {
		return nil, err
	}
	if err := s.enrichMembership(ctx, membership); err != nil {
		return nil, err
	}
	return membership, nil
}

// ListWorkspaceRoles returns all roles defined in a workspace. Requires the
// workspace:manage_roles permission.
func (s *Service) ListWorkspaceRoles(ctx context.Context, workspaceID, userID string) ([]domain.Role, error) {
	if _, err := s.requireWorkspacePermission(ctx, workspaceID, userID, domain.PermissionWorkspaceManageRoles); err != nil {
		return nil, err
	}
	return s.deps.RolesRead.ListByWorkspace(ctx, workspaceID)
}

// ListPermissions returns the global permission registry. Any authenticated user may call this;
// it does not depend on workspace membership.
func (s *Service) ListPermissions(ctx context.Context, userID string) ([]domain.Permission, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, domain.ErrUnauthorized
	}
	return domain.PermissionRegistry(), nil
}

// CreateWorkspaceRole creates a custom role in a workspace with the given permissions.
// Requires workspace:manage_roles. The permission mask can be specified either by name list
// or by raw mask value — if both are provided, the mask value takes precedence.
func (s *Service) CreateWorkspaceRole(ctx context.Context, workspaceID, actorID, name string, permissionNames []string, permissionMask *int64, now time.Time) (*domain.Role, error) {
	if _, err := s.requireWorkspacePermission(ctx, workspaceID, actorID, domain.PermissionWorkspaceManageRoles); err != nil {
		return nil, err
	}
	roleName := strings.TrimSpace(name)
	if roleName == "" {
		return nil, domain.ErrPermissionSetInvalid
	}
	mask, err := resolvePermissionMask(permissionNames, permissionMask)
	if err != nil {
		return nil, err
	}
	roleID, err := s.deps.IDGen.New()
	if err != nil {
		return nil, err
	}
	role := &domain.Role{
		ID:              roleID,
		WorkspaceID:     workspaceID,
		Name:            roleName,
		Type:            domain.RoleTypeCustom,
		PermissionsMask: mask,
		Builtin:         false,
		Status:          domain.RoleStatusActive,
		Version:         1,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := s.deps.RolesWrite.Create(ctx, *role); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return nil, domain.ErrRoleNameConflict
		}
		return nil, err
	}
	return role, nil
}

// UpdateWorkspaceRole modifies a role's name, permissions, or status. Requires
// workspace:manage_roles. Builtin roles cannot be renamed (changing the name of a builtin
// role returns ErrRoleAssignmentConflict). Permission can be set by name list or raw mask.
func (s *Service) UpdateWorkspaceRole(ctx context.Context, workspaceID, roleID, actorID string, name *string, permissionNames []string, permissionMask *int64, status *string, now time.Time) (*domain.Role, error) {
	if _, err := s.requireWorkspacePermission(ctx, workspaceID, actorID, domain.PermissionWorkspaceManageRoles); err != nil {
		return nil, err
	}
	role, err := s.deps.RolesRead.FindByID(ctx, workspaceID, roleID)
	if err != nil {
		return nil, err
	}
	if name != nil && role.Builtin {
		return nil, domain.ErrRoleAssignmentConflict
	}
	if name != nil {
		role.Name = strings.TrimSpace(*name)
	}
	if permissionNames != nil || permissionMask != nil {
		mask, err := resolvePermissionMask(permissionNames, permissionMask)
		if err != nil {
			return nil, err
		}
		role.PermissionsMask = mask
	}
	if status != nil {
		switch normalized := strings.TrimSpace(strings.ToLower(*status)); normalized {
		case string(domain.RoleStatusActive):
			role.Status = domain.RoleStatusActive
		case string(domain.RoleStatusArchived):
			role.Status = domain.RoleStatusArchived
		default:
			return nil, domain.ErrPermissionSetInvalid
		}
	}
	role.Version++
	role.UpdatedAt = now
	if err := s.deps.RolesWrite.Update(ctx, *role); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return nil, domain.ErrRoleNameConflict
		}
		return nil, err
	}
	return role, nil
}

// requireWorkspacePermission loads the user's membership with enriched permissions and
// verifies that the membership grants the specified permission. Returns the enriched
// membership on success, or a domain-specific error (ErrWorkspaceAccessDenied,
// ErrRoleManageDenied, or ErrMembershipManageDenied) on failure.
func (s *Service) requireWorkspacePermission(ctx context.Context, workspaceID, userID, permission string) (*domain.Membership, error) {
	membership, err := s.enrichedMembershipForUser(ctx, workspaceID, userID)
	if err != nil {
		if errors.Is(err, domain.ErrMembershipNotFound) {
			return nil, domain.ErrWorkspaceAccessDenied
		}
		return nil, err
	}
	if !domain.HasPermission(membership.EffectivePermissions, permission) {
		if permission == domain.PermissionWorkspaceManageRoles {
			return nil, domain.ErrRoleManageDenied
		}
		if permission == domain.PermissionWorkspaceManageMembers {
			return nil, domain.ErrMembershipManageDenied
		}
		return nil, domain.ErrWorkspaceAccessDenied
	}
	return membership, nil
}

// enrichedMembershipForUser loads a membership and attaches role IDs, role names, and the
// computed effective permissions. Returns ErrMembershipNotFound if the user is not a member.
func (s *Service) enrichedMembershipForUser(ctx context.Context, workspaceID, userID string) (*domain.Membership, error) {
	membership, err := s.deps.MembershipsRead.FindByWorkspaceAndUser(ctx, workspaceID, userID)
	if err != nil {
		return nil, err
	}
	if membership.Status != domain.MembershipStatusActive {
		return nil, domain.ErrWorkspaceAccessDenied
	}
	if err := s.enrichMembership(ctx, membership); err != nil {
		return nil, err
	}
	return membership, nil
}

// enrichMembership loads the roles for a membership and populates RoleIDs, RoleNames,
// EffectivePermissions, and the legacy Role field (derived from roles if missing).
func (s *Service) enrichMembership(ctx context.Context, membership *domain.Membership) error {
	roles, err := s.deps.RolesRead.ListByMembership(ctx, membership.ID)
	if err != nil {
		return err
	}
	membership.RoleIDs = domain.RoleIDs(roles)
	membership.RoleNames = domain.RoleNames(roles)
	membership.EffectivePermissions = domain.EffectivePermissions(roles)
	if membership.Role == "" {
		membership.Role = domain.LegacyMembershipRole(roles)
	}
	return nil
}

// enrichInvitation loads the roles for an invitation and populates RoleIDs and the legacy
// Role field (derived from roles if missing).
func (s *Service) enrichInvitation(ctx context.Context, invitation *domain.Invitation) error {
	roles, err := s.deps.RolesRead.ListByInvitation(ctx, invitation.ID)
	if err != nil {
		return err
	}
	invitation.RoleIDs = domain.RoleIDs(roles)
	if invitation.Role == "" {
		invitation.Role = domain.LegacyMembershipRole(roles)
	}
	return nil
}

// resolvePermissionMask resolves a permission mask from either a list of permission names
// or a raw mask value. If permissionMask is provided, it takes precedence. Otherwise,
// the names are resolved via the global permission registry.
func resolvePermissionMask(permissionNames []string, permissionMask *int64) (int64, error) {
	if permissionMask != nil {
		if *permissionMask < 0 || *permissionMask&^domain.AllPermissionsMask() != 0 {
			return 0, domain.ErrInvalidPermissionMask
		}
		return *permissionMask, nil
	}
	mask, err := domain.PermissionsMaskFromNames(permissionNames)
	if err != nil {
		if errors.Is(err, domain.ErrPermissionRegistryUnknown) {
			return 0, err
		}
		return 0, domain.ErrPermissionSetInvalid
	}
	return mask, nil
}
