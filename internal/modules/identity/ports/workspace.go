package ports

import (
	"context"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/domain"
)

type WorkspaceReadRepository interface {
	FindByID(ctx context.Context, workspaceID string) (*domain.Workspace, error)
	ListByUser(ctx context.Context, userID string) ([]domain.Workspace, error)
}

type WorkspaceWriteRepository interface {
	Create(ctx context.Context, workspace domain.Workspace) error
}

type RoleReadRepository interface {
	FindByID(ctx context.Context, workspaceID, roleID string) (*domain.Role, error)
	FindByType(ctx context.Context, workspaceID string, roleType domain.RoleType) (*domain.Role, error)
	FindByIDs(ctx context.Context, workspaceID string, roleIDs []string) ([]domain.Role, error)
	ListByWorkspace(ctx context.Context, workspaceID string) ([]domain.Role, error)
	ListByMembership(ctx context.Context, membershipID string) ([]domain.Role, error)
	ListByInvitation(ctx context.Context, invitationID string) ([]domain.Role, error)
	CountMembershipsByRole(ctx context.Context, workspaceID, roleID string) (int, error)
}

type RoleWriteRepository interface {
	Create(ctx context.Context, role domain.Role) error
	Update(ctx context.Context, role domain.Role) error
	ReplaceMembershipRoles(ctx context.Context, membershipID string, roleIDs []string, updatedAt time.Time) error
	ReplaceInvitationRoles(ctx context.Context, invitationID string, roleIDs []string, updatedAt time.Time) error
}

type MembershipReadRepository interface {
	FindByID(ctx context.Context, membershipID string) (*domain.Membership, error)
	FindByWorkspaceAndUser(ctx context.Context, workspaceID, userID string) (*domain.Membership, error)
	ListByWorkspace(ctx context.Context, workspaceID string) ([]domain.Membership, error)
	CountByWorkspaceAndRole(ctx context.Context, workspaceID string, role domain.MembershipRole) (int, error)
}

type MembershipWriteRepository interface {
	Create(ctx context.Context, membership domain.Membership) error
	DeleteByID(ctx context.Context, membershipID string) error
	UpdateRole(ctx context.Context, membershipID string, role domain.MembershipRole, updatedAt time.Time) error
	UpdateStatus(ctx context.Context, membershipID string, status domain.MembershipStatus, updatedAt time.Time) error
}

type InvitationReadRepository interface {
	FindByToken(ctx context.Context, token string) (*domain.Invitation, error)
	ListByWorkspace(ctx context.Context, workspaceID string) ([]domain.Invitation, error)
}

type InvitationWriteRepository interface {
	Create(ctx context.Context, invitation domain.Invitation) error
	UpdateStatus(ctx context.Context, invitationID string, status domain.InvitationStatus, updatedAt time.Time) error
}
