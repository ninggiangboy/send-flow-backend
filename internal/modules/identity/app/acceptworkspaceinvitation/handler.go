package acceptworkspaceinvitation

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/app/usecase"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/transaction"
)

type Command struct {
	Token  string
	UserID string
	Now    time.Time
}

type Handler struct {
	deps usecase.Deps
	log  *slog.Logger
}

func New(deps usecase.Deps) *Handler {
	return &Handler{deps: deps, log: deps.Logger.With("usecase", "accept_workspace_invitation")}
}

func (h *Handler) Execute(ctx context.Context, cmd Command) (*domain.Membership, error) {
	invitation, err := h.deps.InvitationsRead.FindByToken(ctx, cmd.Token)
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
	user, err := h.deps.UsersRead.FindByID(ctx, cmd.UserID)
	if err != nil {
		h.log.Error("failed to find user for invitation acceptance", "user_id", cmd.UserID, "error", err)
		return nil, err
	}
	if !matchEmail(user.Email, invitation.Email) {
		h.log.Warn("invitation email mismatch", "workspace_id", invitation.WorkspaceID, "user_id", cmd.UserID)
		return nil, domain.ErrWorkspaceAccessDenied
	}
	existing, err := h.deps.MembershipsRead.FindByWorkspaceAndUser(ctx, invitation.WorkspaceID, cmd.UserID)
	if err == nil && existing != nil {
		h.log.Warn("user is already a member of workspace", "workspace_id", invitation.WorkspaceID, "user_id", cmd.UserID)
		return nil, domain.ErrInvitationAccepted
	}
	if err != nil && !errors.Is(err, domain.ErrMembershipNotFound) {
		h.log.Error("failed to check existing membership", "workspace_id", invitation.WorkspaceID, "error", err)
		return nil, err
	}
	invitationRoles, err := h.deps.RolesRead.ListByInvitation(ctx, invitation.ID)
	if err != nil {
		h.log.Error("failed to list invitation roles", "workspace_id", invitation.WorkspaceID, "error", err)
		return nil, err
	}
	if len(invitationRoles) == 0 {
		return nil, domain.ErrInvalidRole
	}
	membershipID, err := h.deps.IDGen.New()
	if err != nil {
		h.log.Error("failed to generate membership ID", "error", err)
		return nil, err
	}
	membership := domain.NewMembership(membershipID, invitation.WorkspaceID, cmd.UserID, domain.LegacyMembershipRole(invitationRoles), cmd.Now)
	roleIDs := domain.RoleIDs(invitationRoles)
	persistAcceptance := func(ctx context.Context) error {
		if err := h.deps.MembershipsWrite.Create(ctx, membership); err != nil {
			return err
		}
		if err := h.deps.RolesWrite.ReplaceMembershipRoles(ctx, membership.ID, roleIDs, cmd.Now); err != nil {
			return err
		}
		return h.deps.InvitationsWrite.UpdateStatus(ctx, invitation.ID, domain.InvitationStatusAccepted, cmd.Now)
	}
	if err := transaction.RunInTx(ctx, h.deps.UnitOfWork, persistAcceptance); err != nil {
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
