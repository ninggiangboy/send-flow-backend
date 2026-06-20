package acceptworkspaceinvitation

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/ports"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/transaction"
)

type Options struct {
	InvitationsWrite ports.InvitationWriteRepository
	UsersWrite       ports.UserWriteRepository
	MembershipsWrite ports.MembershipWriteRepository
	RolesWrite       ports.RoleWriteRepository
	IdGen            ports.IDGenerator
	UnitOfWork       ports.UnitOfWork
	Logger           *slog.Logger
}

type Command struct {
	Token  string
	UserID string
	Now    time.Time
}

type Handler struct {
	invitationsWrite ports.InvitationWriteRepository
	usersWrite       ports.UserWriteRepository
	membershipsWrite ports.MembershipWriteRepository
	rolesWrite       ports.RoleWriteRepository
	idGen            ports.IDGenerator
	unitOfWork       ports.UnitOfWork
	log              *slog.Logger
}

func New(opts Options) *Handler {
	return &Handler{
		invitationsWrite: opts.InvitationsWrite,
		usersWrite:       opts.UsersWrite,
		membershipsWrite: opts.MembershipsWrite,
		rolesWrite:       opts.RolesWrite,
		idGen:            opts.IdGen,
		unitOfWork:       opts.UnitOfWork,
		log:              opts.Logger.With("usecase", "accept_workspace_invitation"),
	}
}

func (h *Handler) Execute(ctx context.Context, cmd Command) (*domain.Membership, error) {
	invitation, err := h.invitationsWrite.FindByToken(ctx, cmd.Token)
	if err != nil {
		if errors.Is(err, domain.ErrInvitationNotFound) {
			h.log.Warn("invitation not found or invalid token")
			return nil, domain.ErrInvitationNotFound
		}
		h.log.Error("failed to find invitation by token", "error", err)
		return nil, err
	}
	if invitation.IsExpired(cmd.Now) {
		h.log.Warn("invitation expired", "workspace_id", invitation.WorkspaceID)
		return nil, domain.ErrInvitationExpired
	}
	if invitation.IsAccepted() {
		h.log.Warn("invitation already accepted", "workspace_id", invitation.WorkspaceID)
		return nil, domain.ErrInvitationAccepted
	}
	user, err := h.usersWrite.FindByID(ctx, cmd.UserID)
	if err != nil {
		h.log.Error("failed to find user for invitation acceptance", "user_id", cmd.UserID, "error", err)
		return nil, err
	}
	if !matchEmail(user.Email, invitation.Email) {
		h.log.Warn("invitation email mismatch", "workspace_id", invitation.WorkspaceID, "user_id", cmd.UserID)
		return nil, domain.ErrWorkspaceAccessDenied
	}
	existing, err := h.membershipsWrite.FindByWorkspaceAndUser(ctx, invitation.WorkspaceID, cmd.UserID)
	if err == nil && existing != nil {
		h.log.Warn("user is already a member of workspace", "workspace_id", invitation.WorkspaceID, "user_id", cmd.UserID)
		return nil, domain.ErrInvitationAccepted
	}
	if err != nil && !errors.Is(err, domain.ErrMembershipNotFound) {
		h.log.Error("failed to check existing membership", "workspace_id", invitation.WorkspaceID, "error", err)
		return nil, err
	}
	invitationRoles, err := h.rolesWrite.ListByInvitation(ctx, invitation.ID)
	if err != nil {
		h.log.Error("failed to list invitation roles", "workspace_id", invitation.WorkspaceID, "error", err)
		return nil, err
	}
	if len(invitationRoles) == 0 {
		return nil, domain.ErrInvalidRole
	}
	membershipID, err := h.idGen.New()
	if err != nil {
		h.log.Error("failed to generate membership ID", "error", err)
		return nil, err
	}
	membership := domain.NewMembership(membershipID, invitation.WorkspaceID, cmd.UserID, domain.LegacyMembershipRole(invitationRoles), cmd.Now)
	roleIDs := domain.RoleIDs(invitationRoles)
	persistAcceptance := func(ctx context.Context) error {
		if err := h.membershipsWrite.Create(ctx, membership); err != nil {
			return err
		}
		if err := h.rolesWrite.ReplaceMembershipRoles(ctx, membership.ID, roleIDs, cmd.Now); err != nil {
			return err
		}
		return h.invitationsWrite.UpdateStatus(ctx, invitation.ID, domain.InvitationStatusAccepted, cmd.Now)
	}
	if err := transaction.RunInTx(ctx, h.unitOfWork, persistAcceptance); err != nil {
		h.log.Error("failed to accept invitation", "workspace_id", invitation.WorkspaceID, "error", err)
		return nil, err
	}
	membership.RoleIDs = roleIDs
	membership.RoleNames = domain.RoleNames(invitationRoles)
	membership.EffectivePermissions = domain.EffectivePermissions(invitationRoles)
	h.log.Info("workspace invitation accepted", "workspace_id", invitation.WorkspaceID, "user_id", cmd.UserID)
	return &membership, nil
}

func matchEmail(userEmail, invitationEmail string) bool {
	return strings.EqualFold(userEmail, invitationEmail)
}
