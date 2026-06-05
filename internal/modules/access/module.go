package access

const Name = "access"

const Purpose = "Manage permission evaluation, roles, API keys, and scopes."

var OwnedData = []string{
	"roles",
	"membership_roles",
	"permission_registry",
	"api_keys",
}

var Commands = []string{
	"CreateRole",
	"UpdateRolePermissions",
	"AssignMemberRoles",
	"CreateAPIKey",
	"RevokeAPIKey",
}

var Queries = []string{
	"CheckPermission",
	"ResolveEffectivePermissions",
	"AuthenticateAPIKey",
	"CheckAPIKeyScope",
}

var Events = []string{
	"access.role.permissions_changed.v1",
	"access.api_key.created.v1",
	"access.api_key.revoked.v1",
}
