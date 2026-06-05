package domain

import "time"

type InvitationStatus string

const (
	InvitationStatusPending  InvitationStatus = "pending"
	InvitationStatusAccepted InvitationStatus = "accepted"
	InvitationStatusExpired  InvitationStatus = "expired"
)

type Invitation struct {
	ID          string
	WorkspaceID string
	Email       string
	Token       string
	Role        MembershipRole
	RoleIDs     []string
	Status      InvitationStatus
	ExpiresAt   time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func NewInvitation(id, workspaceID, email, token string, role MembershipRole, expiresAt, now time.Time) Invitation {
	return Invitation{
		ID:          id,
		WorkspaceID: workspaceID,
		Email:       email,
		Token:       token,
		Role:        role,
		Status:      InvitationStatusPending,
		ExpiresAt:   expiresAt,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func (i Invitation) IsExpired(now time.Time) bool {
	return now.After(i.ExpiresAt)
}

func (i Invitation) IsAccepted() bool {
	return i.Status == InvitationStatusAccepted
}
