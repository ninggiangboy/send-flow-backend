package domain

import (
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/identity/contracts"
)

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

func (e UserRegistered) EventName() string     { return contracts.EventUserRegisteredV1 }
func (e UserRegistered) OccurredAt() time.Time { return e.At }

type ExternalAccountLinked struct {
	AccountID      string
	UserID         string
	Provider       string
	ProviderUserID string
	At             time.Time
}

func (e ExternalAccountLinked) EventName() string     { return contracts.EventExternalAccountLinkedV1 }
func (e ExternalAccountLinked) OccurredAt() time.Time { return e.At }

type WorkspaceCreated struct {
	WorkspaceID string
	CreatorID   string
	Name        string
	At          time.Time
}

func (e WorkspaceCreated) EventName() string     { return contracts.EventWorkspaceCreatedV1 }
func (e WorkspaceCreated) OccurredAt() time.Time { return e.At }

type WorkspaceMemberInvited struct {
	WorkspaceID string
	Email       string
	Role        string
	InvitedBy   string
	At          time.Time
}

func (e WorkspaceMemberInvited) EventName() string     { return contracts.EventWorkspaceMemberInvitedV1 }
func (e WorkspaceMemberInvited) OccurredAt() time.Time { return e.At }

type WorkspaceMemberJoined struct {
	WorkspaceID  string
	UserID       string
	MembershipID string
	At           time.Time
}

func (e WorkspaceMemberJoined) EventName() string     { return contracts.EventWorkspaceMemberJoinedV1 }
func (e WorkspaceMemberJoined) OccurredAt() time.Time { return e.At }
