package contracts

const (
	EventUserRegisteredV1          = "identity.user.registered.v1"
	EventExternalAccountLinkedV1   = "identity.user.external_account_linked.v1"
	EventWorkspaceCreatedV1        = "identity.workspace.created.v1"
	EventWorkspaceMemberInvitedV1  = "identity.workspace.member_invited.v1"
	EventWorkspaceMemberJoinedV1   = "identity.workspace.member_joined.v1"
)

type UserRegisteredPayload struct {
	UserID     string `json:"user_id"`
	Email      string `json:"email"`
	AuthMethod string `json:"auth_method"`
	At         string `json:"at"`
}

type ExternalAccountLinkedPayload struct {
	AccountID      string `json:"account_id"`
	UserID         string `json:"user_id"`
	Provider       string `json:"provider"`
	ProviderUserID string `json:"provider_user_id"`
	At             string `json:"at"`
}

type WorkspaceCreatedPayload struct {
	WorkspaceID string `json:"workspace_id"`
	CreatorID   string `json:"creator_id"`
	Name        string `json:"name"`
	At          string `json:"at"`
}

type WorkspaceMemberInvitedPayload struct {
	WorkspaceID string `json:"workspace_id"`
	Email       string `json:"email"`
	Role        string `json:"role"`
	InvitedBy   string `json:"invited_by"`
	At          string `json:"at"`
}

type WorkspaceMemberJoinedPayload struct {
	WorkspaceID  string `json:"workspace_id"`
	UserID       string `json:"user_id"`
	MembershipID string `json:"membership_id"`
	At           string `json:"at"`
}
