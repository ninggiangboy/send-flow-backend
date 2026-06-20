package removeworkspacemember

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/ports"
)

type Options struct {
	MembershipsWrite ports.MembershipWriteRepository
	RolesWrite       ports.RoleWriteRepository
	Logger           *slog.Logger
}

type Command struct {
	WorkspaceID  string
	MembershipID string
	RemoverID    string
	Now          time.Time
}

type Handler struct {
	membershipsWrite ports.MembershipWriteRepository
	rolesWrite       ports.RoleWriteRepository
	log              *slog.Logger
}

func New(opts Options) *Handler {
	return &Handler{
		membershipsWrite: opts.MembershipsWrite,
		rolesWrite:       opts.RolesWrite,
		log:              opts.Logger.With("usecase", "remove_workspace_member"),
	}
}

func (h *Handler) Execute(ctx context.Context, cmd Command) error {
	removerMembership, err := h.membershipsWrite.FindByWorkspaceAndUser(ctx, cmd.WorkspaceID, cmd.RemoverID)
	if err != nil {
		if errors.Is(err, domain.ErrMembershipNotFound) {
			h.log.Warn("member removal denied: remover is not a member", "workspace_id", cmd.WorkspaceID, "remover_id", cmd.RemoverID)
			return domain.ErrWorkspaceAccessDenied
		}
		h.log.Error("failed to find remover membership", "workspace_id", cmd.WorkspaceID, "error", err)
		return err
	}
	removerRoles, err := h.rolesWrite.ListByMembership(ctx, removerMembership.ID)
	if err != nil {
		h.log.Error("failed to list remover roles", "workspace_id", cmd.WorkspaceID, "error", err)
		return err
	}
	if !domain.HasPermission(domain.EffectivePermissions(removerRoles), domain.PermissionWorkspaceManageMembers) {
		h.log.Warn("member removal denied: insufficient permissions", "workspace_id", cmd.WorkspaceID, "remover_id", cmd.RemoverID)
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
	if targetMembership.ID == removerMembership.ID {
		h.log.Warn("self-removal attempted", "workspace_id", cmd.WorkspaceID, "remover_id", cmd.RemoverID)
		return domain.ErrMembershipManageDenied
	}
	targetRoles, err := h.rolesWrite.ListByMembership(ctx, targetMembership.ID)
	if err != nil {
		h.log.Error("failed to list target roles", "workspace_id", cmd.WorkspaceID, "error", err)
		return err
	}
	if domain.LegacyMembershipRole(targetRoles) == domain.MembershipRoleOwner {
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
			h.log.Warn("cannot remove last owner from workspace", "workspace_id", cmd.WorkspaceID)
			return domain.ErrLastOwnerCannotBeRemoved
		}
	}
	h.log.Info("workspace member removed", "workspace_id", cmd.WorkspaceID, "membership_id", cmd.MembershipID)
	return h.membershipsWrite.DeleteByID(ctx, cmd.MembershipID)
}
