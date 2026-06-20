package updateworkspacememberrole

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/ports"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/transaction"
)

type Options struct {
	MembershipsWrite ports.MembershipWriteRepository
	RolesWrite       ports.RoleWriteRepository
	UnitOfWork       ports.UnitOfWork
	Logger           *slog.Logger
}

type Command struct {
	WorkspaceID  string
	MembershipID string
	RoleIDs      []string
	UpdaterID    string
	Now          time.Time
}

type Handler struct {
	membershipsWrite ports.MembershipWriteRepository
	rolesWrite       ports.RoleWriteRepository
	unitOfWork       ports.UnitOfWork
	log              *slog.Logger
}

func New(opts Options) *Handler {
	return &Handler{
		membershipsWrite: opts.MembershipsWrite,
		rolesWrite:       opts.RolesWrite,
		unitOfWork:       opts.UnitOfWork,
		log:              opts.Logger.With("usecase", "update_workspace_member_role"),
	}
}

func (h *Handler) Execute(ctx context.Context, cmd Command) error {
	updaterMembership, err := h.membershipsWrite.FindByWorkspaceAndUser(ctx, cmd.WorkspaceID, cmd.UpdaterID)
	if err != nil {
		if errors.Is(err, domain.ErrMembershipNotFound) {
			h.log.Warn("role update denied: updater is not a member", "workspace_id", cmd.WorkspaceID, "updater_id", cmd.UpdaterID)
			return domain.ErrWorkspaceAccessDenied
		}
		h.log.Error("failed to find updater membership", "workspace_id", cmd.WorkspaceID, "error", err)
		return err
	}
	updaterRoles, err := h.rolesWrite.ListByMembership(ctx, updaterMembership.ID)
	if err != nil {
		h.log.Error("failed to list updater roles", "workspace_id", cmd.WorkspaceID, "error", err)
		return err
	}
	if !domain.HasPermission(domain.EffectivePermissions(updaterRoles), domain.PermissionWorkspaceManageRoles) {
		h.log.Warn("role update denied: insufficient permissions", "workspace_id", cmd.WorkspaceID, "updater_id", cmd.UpdaterID)
		return domain.ErrMembershipManageDenied
	}
	targetMembership, err := h.membershipsWrite.FindByID(ctx, cmd.MembershipID)
	if err != nil {
		h.log.Error("failed to find target membership", "workspace_id", cmd.WorkspaceID, "membership_id", cmd.MembershipID, "error", err)
		return err
	}
	if targetMembership.WorkspaceID != cmd.WorkspaceID {
		h.log.Warn("target membership does not belong to workspace", "workspace_id", cmd.WorkspaceID, "membership_id", cmd.MembershipID)
		return domain.ErrMembershipNotFound
	}
	if len(cmd.RoleIDs) == 0 {
		return domain.ErrInvalidRole
	}
	roles, err := h.rolesWrite.FindByIDs(ctx, cmd.WorkspaceID, cmd.RoleIDs)
	if err != nil {
		h.log.Error("failed to find roles", "workspace_id", cmd.WorkspaceID, "error", err)
		return err
	}
	if len(roles) != len(cmd.RoleIDs) {
		return domain.ErrInvalidRole
	}
	for _, role := range roles {
		if role.Status != domain.RoleStatusActive {
			return domain.ErrInvalidRole
		}
	}
	targetRoles, err := h.rolesWrite.ListByMembership(ctx, targetMembership.ID)
	if err != nil {
		h.log.Error("failed to list target roles", "workspace_id", cmd.WorkspaceID, "error", err)
		return err
	}
	if domain.LegacyMembershipRole(targetRoles) == domain.MembershipRoleOwner && domain.LegacyMembershipRole(roles) != domain.MembershipRoleOwner {
		ownerRole, err := h.rolesWrite.FindByType(ctx, cmd.WorkspaceID, domain.RoleTypeOwner)
		if err != nil {
			h.log.Error("failed to find owner role", "workspace_id", cmd.WorkspaceID, "error", err)
			return err
		}
		count, err := h.rolesWrite.CountMembershipsByRole(ctx, cmd.WorkspaceID, ownerRole.ID)
		if err != nil {
			h.log.Error("failed to count owner memberships", "workspace_id", cmd.WorkspaceID, "error", err)
			return err
		}
		if count <= 1 {
			h.log.Warn("cannot demote last owner", "workspace_id", cmd.WorkspaceID, "membership_id", cmd.MembershipID)
			return domain.ErrLastOwnerCannotBeRemoved
		}
	}
	legacyRole := domain.LegacyMembershipRole(roles)
	updateAll := func(ctx context.Context) error {
		if err := h.rolesWrite.ReplaceMembershipRoles(ctx, cmd.MembershipID, domain.RoleIDs(roles), cmd.Now); err != nil {
			return err
		}
		return h.membershipsWrite.UpdateRole(ctx, cmd.MembershipID, legacyRole, cmd.Now)
	}
	if err := transaction.RunInTx(ctx, h.unitOfWork, updateAll); err != nil {
		h.log.Error("failed to update member role", "workspace_id", cmd.WorkspaceID, "error", err)
		return err
	}
	h.log.Info("workspace member role updated", "workspace_id", cmd.WorkspaceID, "membership_id", cmd.MembershipID)
	return nil
}
