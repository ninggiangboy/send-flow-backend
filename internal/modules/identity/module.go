package identity

const Name = "identity"

const Purpose = "Manage global identity, tenant boundaries, membership, and session context."

var OwnedData = []string{
	"users",
	"external_auth_accounts",
	"workspaces",
	"workspace_memberships",
	"workspace_invitations",
	"sessions",
}

var Commands = []string{
	"RegisterUser",
	"Login",
	"LinkExternalAuthAccount",
	"CreateWorkspace",
	"InviteWorkspaceMember",
	"AcceptWorkspaceInvitation",
	"SwitchActiveWorkspace",
	"RevokeSession",
}

var Queries = []string{
	"GetCurrentSession",
	"GetWorkspaceMembership",
	"ListWorkspaceMembers",
	"ResolveWorkspaceContext",
}

var Events = []string{
	"identity.user.registered.v1",
	"identity.user.external_account_linked.v1",
	"identity.workspace.created.v1",
	"identity.workspace.member_invited.v1",
	"identity.workspace.member_joined.v1",
}
