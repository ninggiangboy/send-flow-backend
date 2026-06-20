package inviteworkspacemember

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/usecase"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/contracts"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/ports"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/transaction"
)

type Options struct {
	MembershipsWrite ports.MembershipWriteRepository
	RolesWrite       ports.RoleWriteRepository
	UsersWrite       ports.UserWriteRepository
	IdGen            ports.IDGenerator
	InvitationsWrite ports.InvitationWriteRepository
	OutboxWriter     ports.OutboxWriter
	UnitOfWork       ports.UnitOfWork
	Logger           *slog.Logger
}

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
	membershipsWrite ports.MembershipWriteRepository
	rolesWrite       ports.RoleWriteRepository
	usersWrite       ports.UserWriteRepository
	idGen            ports.IDGenerator
	invitationsWrite ports.InvitationWriteRepository
	outboxWriter     ports.OutboxWriter
	unitOfWork       ports.UnitOfWork
	log              *slog.Logger
}

func New(opts Options) *Handler {
	return &Handler{
		membershipsWrite: opts.MembershipsWrite,
		rolesWrite:       opts.RolesWrite,
		usersWrite:       opts.UsersWrite,
		idGen:            opts.IdGen,
		invitationsWrite: opts.InvitationsWrite,
		outboxWriter:     opts.OutboxWriter,
		unitOfWork:       opts.UnitOfWork,
		log:              opts.Logger.With("usecase", "invite_workspace_member"),
	}
}

func (h *Handler) Execute(ctx context.Context, cmd Command) (*Result, error) {
	inviterMembership, err := h.membershipsWrite.FindByWorkspaceAndUser(ctx, cmd.WorkspaceID, cmd.InviterID)
	if err != nil {
		if errors.Is(err, domain.ErrMembershipNotFound) {
			h.log.Warn("invitation denied: inviter is not a member", "workspace_id", cmd.WorkspaceID, "inviter_id", cmd.InviterID)
			return nil, domain.ErrWorkspaceAccessDenied
		}
		h.log.Error("failed to find inviter membership", "workspace_id", cmd.WorkspaceID, "error", err)
		return nil, err
	}
	inviterRoles, err := h.rolesWrite.ListByMembership(ctx, inviterMembership.ID)
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
	assignedRoles, err := h.rolesWrite.FindByIDs(ctx, cmd.WorkspaceID, cmd.RoleIDs)
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
	inviter, err := h.usersWrite.FindByID(ctx, cmd.InviterID)
	if err != nil {
		h.log.Error("failed to lookup inviter", "inviter_id", cmd.InviterID, "error", err)
		return nil, err
	}
	invitedUser, err := h.usersWrite.FindByEmail(ctx, email)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		h.log.Error("failed to lookup invited user", "workspace_id", cmd.WorkspaceID, "error", err)
		return nil, err
	}
	if invitedUser != nil {
		existingMembership, err := h.membershipsWrite.FindByWorkspaceAndUser(ctx, cmd.WorkspaceID, invitedUser.ID)
		if err == nil && existingMembership != nil {
			h.log.Warn("invitation denied: user is already a member", "workspace_id", cmd.WorkspaceID)
			return nil, domain.ErrInvitationPayloadInvalid
		}
		if err != nil && !errors.Is(err, domain.ErrMembershipNotFound) {
			h.log.Error("failed to check existing membership", "workspace_id", cmd.WorkspaceID, "error", err)
			return nil, err
		}
	}
	token, err := h.idGen.New()
	if err != nil {
		h.log.Error("failed to generate invitation token", "workspace_id", cmd.WorkspaceID, "error", err)
		return nil, err
	}
	expiresAt := cmd.Now.Add(7 * 24 * time.Hour)
	legacyRole := domain.LegacyMembershipRole(assignedRoles)
	roleIDs := domain.RoleIDs(assignedRoles)
	invitationID, err := h.idGen.New()
	if err != nil {
		h.log.Error("failed to generate invitation ID", "workspace_id", cmd.WorkspaceID, "error", err)
		return nil, err
	}
	invitation := domain.NewInvitation(invitationID, cmd.WorkspaceID, email, token, legacyRole, expiresAt, cmd.Now)
	invitation.Role = legacyRole
	invitation.RoleIDs = roleIDs
	persistInvitation := func(txCtx context.Context) error {
		if err := h.invitationsWrite.Create(txCtx, invitation); err != nil {
			return err
		}
		if err := h.rolesWrite.ReplaceInvitationRoles(txCtx, invitation.ID, roleIDs, cmd.Now); err != nil {
			return err
		}
		payload := map[string]string{
			"workspace_id":     cmd.WorkspaceID,
			"email":            email,
			"role":             string(legacyRole),
			"invited_by":       cmd.InviterID,
			"invited_by_email": inviter.Email,
			"at":               cmd.Now.Format(time.RFC3339),
		}
		if err := usecase.EmitEvent(txCtx, h.outboxWriter, h.idGen, contracts.EventWorkspaceMemberInvitedV1, "invitation", invitation.ID, cmd.WorkspaceID, payload, cmd.Now); err != nil {
			h.log.Error("failed to emit member invited event", "error", err)
			return err
		}
		return nil
	}
	if err := transaction.RunInTx(ctx, h.unitOfWork, persistInvitation); err != nil {
		h.log.Error("failed to create invitation", "workspace_id", cmd.WorkspaceID, "error", err)
		return nil, err
	}
	h.log.Info("workspace invitation created", "workspace_id", cmd.WorkspaceID, "inviter_id", cmd.InviterID)

	return &Result{Invitation: &invitation}, nil
}
