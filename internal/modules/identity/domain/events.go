package domain

import "time"

type Event interface {
	EventName() string
	OccurredAt() time.Time
}

type UserRegistered struct {
	UserID     string
	Email      string
	AuthMethod string
	At         time.Time
}

func (e UserRegistered) EventName() string     { return "identity.user.registered.v1" }
func (e UserRegistered) OccurredAt() time.Time { return e.At }

type ExternalAccountLinked struct {
	AccountID      string
	UserID         string
	Provider       string
	ProviderUserID string
	At             time.Time
}

func (e ExternalAccountLinked) EventName() string     { return "identity.user.external_account_linked.v1" }
func (e ExternalAccountLinked) OccurredAt() time.Time { return e.At }

type WorkspaceCreated struct {
	WorkspaceID string
	CreatorID   string
	Name        string
	At          time.Time
}

func (e WorkspaceCreated) EventName() string     { return "identity.workspace.created.v1" }
func (e WorkspaceCreated) OccurredAt() time.Time { return e.At }

type WorkspaceMemberInvited struct {
	WorkspaceID string
	Email       string
	Role        string
	InvitedBy   string
	At          time.Time
}

func (e WorkspaceMemberInvited) EventName() string     { return "identity.workspace.member_invited.v1" }
func (e WorkspaceMemberInvited) OccurredAt() time.Time { return e.At }

type WorkspaceMemberJoined struct {
	WorkspaceID  string
	UserID       string
	MembershipID string
	At           time.Time
}

func (e WorkspaceMemberJoined) EventName() string     { return "identity.workspace.member_joined.v1" }
func (e WorkspaceMemberJoined) OccurredAt() time.Time { return e.At }
