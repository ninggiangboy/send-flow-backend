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
	MembershipsRead  ports.MembershipReadRepository
	MembershipsWrite ports.MembershipWriteRepository
	RolesRead        ports.RoleReadRepository
	Logger           *slog.Logger
}

type Command struct {
	WorkspaceID  string
	MembershipID string
	RemoverID    string
	Now          time.Time
}

type Handler struct {
	membershipsRead  ports.MembershipReadRepository
	membershipsWrite ports.MembershipWriteRepository
	rolesRead        ports.RoleReadRepository
	log              *slog.Logger
}

func New(opts Options) *Handler {
	return &Handler{
		membershipsRead:  opts.MembershipsRead,
		membershipsWrite: opts.MembershipsWrite,
		rolesRead:        opts.RolesRead,
		log:              opts.Logger.With("usecase", "remove_workspace_member"),
	}
}

func (h *Handler) Execute(ctx context.Context, cmd Command) error {
	removerMembership, err := h.membershipsRead.FindByWorkspaceAndUser(ctx, cmd.WorkspaceID, cmd.RemoverID)
	if err != nil {
		if errors.Is(err, domain.ErrMembershipNotFound) {
			h.log.Warn("member removal denied: remover is not a member", "workspace_id", cmd.WorkspaceID, "remover_id", cmd.RemoverID)
			return domain.ErrWorkspaceAccessDenied
		}
		h.log.Error("failed to find remover membership", "workspace_id", cmd.WorkspaceID, "error", err)
		return err
	}
	removerRoles, err := h.rolesRead.ListByMembership(ctx, removerMembership.ID)
	if err != nil {
		h.log.Error("failed to list remover roles", "workspace_id", cmd.WorkspaceID, "error", err)
		return err
	}
	if !domain.HasPermission(domain.EffectivePermissions(removerRoles), domain.PermissionWorkspaceManageMembers) {
		h.log.Warn("member removal denied: insufficient permissions", "workspace_id", cmd.WorkspaceID, "remover_id", cmd.RemoverID)
		return domain.ErrMembershipManageDenied
	}
	targetMembership, err := h.membershipsRead.FindByID(ctx, cmd.MembershipID)
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
	targetRoles, err := h.rolesRead.ListByMembership(ctx, targetMembership.ID)
	if err != nil {
		h.log.Error("failed to list target roles", "workspace_id", cmd.WorkspaceID, "error", err)
		return err
	}
	if domain.LegacyMembershipRole(targetRoles) == domain.MembershipRoleOwner {
		ownerRole, err := h.rolesRead.FindByType(ctx, cmd.WorkspaceID, domain.RoleTypeOwner)
		if err != nil {
			h.log.Error("failed to find owner role", "workspace_id", cmd.WorkspaceID, "error", err)
			return err
		}
		count, err := h.rolesRead.CountMembershipsByRole(ctx, cmd.WorkspaceID, ownerRole.ID)
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
