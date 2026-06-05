package domain

import "time"

type MembershipRole string

const (
	MembershipRoleOwner  MembershipRole = "Owner"
	MembershipRoleAdmin  MembershipRole = "Admin"
	MembershipRoleMember MembershipRole = "Member"
)

type MembershipStatus string

const (
	MembershipStatusActive    MembershipStatus = "active"
	MembershipStatusInvited   MembershipStatus = "invited"
	MembershipStatusSuspended MembershipStatus = "suspended"
)

type Membership struct {
	ID                   string
	WorkspaceID          string
	UserID               string
	UserEmail            *string
	Role                 MembershipRole
	RoleIDs              []string
	RoleNames            []string
	EffectivePermissions []string
	Status               MembershipStatus
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

func NewMembership(id, workspaceID, userID string, role MembershipRole, now time.Time) Membership {
	return Membership{
		ID:          id,
		WorkspaceID: workspaceID,
		UserID:      userID,
		Role:        role,
		Status:      MembershipStatusActive,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func NewInvitedMembership(id, workspaceID, userID string, role MembershipRole, now time.Time) Membership {
	return Membership{
		ID:          id,
		WorkspaceID: workspaceID,
		UserID:      userID,
		Role:        role,
		Status:      MembershipStatusInvited,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}
