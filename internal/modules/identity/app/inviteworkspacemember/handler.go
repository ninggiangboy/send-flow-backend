package inviteworkspacemember

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/usecase"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/id"
)

type Command struct {
	WorkspaceID string
	Email       string
	RoleIDs     []string
	InviterID   string
	Now         time.Time
}

type Result struct {
	Invitation *domain.Invitation `json:"invitation,omitempty"`
}

type Handler struct {
	deps usecase.Deps
	log  *slog.Logger
}

func New(deps usecase.Deps) *Handler {
	return &Handler{deps: deps, log: deps.Logger.With("usecase", "invite_workspace_member")}
}

func (h *Handler) Execute(ctx context.Context, cmd Command) (*Result, error) {
	inviterMembership, err := h.deps.MembershipsRead.FindByWorkspaceAndUser(ctx, cmd.WorkspaceID, cmd.InviterID)
	if err != nil {
		if errors.Is(err, domain.ErrMembershipNotFound) {
			h.log.Warn("invitation denied: inviter is not a member", "workspace_id", cmd.WorkspaceID, "inviter_id", cmd.InviterID)
			return nil, domain.ErrWorkspaceAccessDenied
		}
		h.log.Error("failed to find inviter membership", "workspace_id", cmd.WorkspaceID, "error", err)
		return nil, err
	}
	inviterRoles, err := h.deps.RolesRead.ListByMembership(ctx, inviterMembership.ID)
	if err != nil {
		h.log.Error("failed to list inviter roles", "workspace_id", cmd.WorkspaceID, "error", err)
		return nil, err
	}
	if !domain.HasPermission(domain.EffectivePermissions(inviterRoles), domain.PermissionWorkspaceManageMembers) {
		h.log.Warn("invitation denied: inviter lacks manage_members permission", "workspace_id", cmd.WorkspaceID, "inviter_id", cmd.InviterID)
		return nil, domain.ErrMembershipManageDenied
	}
	if len(cmd.RoleIDs) == 0 {
		return nil, domain.ErrInvitationPayloadInvalid
	}
	assignedRoles, err := h.deps.RolesRead.FindByIDs(ctx, cmd.WorkspaceID, cmd.RoleIDs)
	if err != nil {
		h.log.Error("failed to find roles for invitation", "workspace_id", cmd.WorkspaceID, "error", err)
		return nil, err
	}
	if len(assignedRoles) != len(cmd.RoleIDs) {
		return nil, domain.ErrInvalidRole
	}
	for _, role := range assignedRoles {
		if role.Status != domain.RoleStatusActive {
			return nil, domain.ErrInvalidRole
		}
	}
	email := strings.TrimSpace(strings.ToLower(cmd.Email))
	if email == "" {
		return nil, domain.ErrInvitationPayloadInvalid
	}
	invitedUser, err := h.deps.UsersRead.FindByEmail(ctx, email)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		h.log.Error("failed to lookup invited user", "workspace_id", cmd.WorkspaceID, "error", err)
		return nil, err
	}
	if invitedUser != nil {
		existingMembership, err := h.deps.MembershipsRead.FindByWorkspaceAndUser(ctx, cmd.WorkspaceID, invitedUser.ID)
		if err == nil && existingMembership != nil {
			h.log.Warn("invitation denied: user is already a member", "workspace_id", cmd.WorkspaceID)
			return nil, domain.ErrInvitationPayloadInvalid
		}
		if err != nil && !errors.Is(err, domain.ErrMembershipNotFound) {
			h.log.Error("failed to check existing membership", "workspace_id", cmd.WorkspaceID, "error", err)
			return nil, err
		}
	}
	token := id.Must(id.NewUUIDGenerator())
	expiresAt := cmd.Now.Add(7 * 24 * time.Hour)
	legacyRole := domain.LegacyMembershipRole(assignedRoles)
	roleIDs := domain.RoleIDs(assignedRoles)
	invitation := domain.NewInvitation(id.Must(id.NewUUIDGenerator()), cmd.WorkspaceID, email, token, legacyRole, expiresAt, cmd.Now)
	invitation.Role = legacyRole
	invitation.RoleIDs = roleIDs
	persistInvitation := func(ctx context.Context) error {
		if err := h.deps.InvitationsWrite.Create(ctx, invitation); err != nil {
			return err
		}
		return h.deps.RolesWrite.ReplaceInvitationRoles(ctx, invitation.ID, roleIDs, cmd.Now)
	}
	if h.deps.UnitOfWork != nil {
		if err := h.deps.UnitOfWork.WithinTx(ctx, persistInvitation); err != nil {
			h.log.Error("failed to create invitation", "workspace_id", cmd.WorkspaceID, "error", err)
			return nil, err
		}
	} else {
		if err := persistInvitation(ctx); err != nil {
			h.log.Error("failed to create invitation", "workspace_id", cmd.WorkspaceID, "error", err)
			return nil, err
		}
	}
	h.log.Info("workspace invitation created", "workspace_id", cmd.WorkspaceID, "inviter_id", cmd.InviterID)
	return &Result{Invitation: &invitation}, nil
}
