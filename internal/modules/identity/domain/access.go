package domain

import (
	"slices"
	"sort"
	"strings"
	"time"
)

type Permission struct {
	Bit  int64
	Name string
}

type RoleStatus string

const (
	RoleStatusActive   RoleStatus = "active"
	RoleStatusArchived RoleStatus = "archived"
)

type RoleType string

const (
	RoleTypeOwner   RoleType = "owner"
	RoleTypeMember  RoleType = "member"
	RoleTypeCustom  RoleType = "custom"
)

const (
	PermissionWorkspaceRead          = "workspace.read"
	PermissionWorkspaceManageMembers = "workspace.manage_members"
	PermissionWorkspaceManageRoles   = "workspace.manage_roles"
	PermissionWorkspaceManageSetting = "workspace.manage_settings"
)

var permissionRegistry = []Permission{
	{Bit: 1 << 0, Name: PermissionWorkspaceRead},
	{Bit: 1 << 1, Name: PermissionWorkspaceManageMembers},
	{Bit: 1 << 2, Name: PermissionWorkspaceManageRoles},
	{Bit: 1 << 3, Name: PermissionWorkspaceManageSetting},
}

type Role struct {
	ID              string
	WorkspaceID     string
	Name            string
	Type            RoleType
	PermissionsMask int64
	Builtin         bool
	Status          RoleStatus
	Version         int
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (r Role) PermissionNames() []string {
	return PermissionNamesFromMask(r.PermissionsMask)
}

func PermissionRegistry() []Permission {
	return slices.Clone(permissionRegistry)
}

func PermissionNamesFromMask(mask int64) []string {
	names := make([]string, 0, len(permissionRegistry))
	for _, permission := range permissionRegistry {
		if mask&permission.Bit != 0 {
			names = append(names, permission.Name)
		}
	}
	sort.Strings(names)
	return names
}

func EffectivePermissions(roles []Role) []string {
	var mask int64
	for _, role := range roles {
		mask |= role.PermissionsMask
	}
	return PermissionNamesFromMask(mask)
}

func RoleIDs(roles []Role) []string {
	ids := make([]string, 0, len(roles))
	for _, role := range roles {
		ids = append(ids, role.ID)
	}
	return ids
}

func RoleNames(roles []Role) []string {
	names := make([]string, 0, len(roles))
	for _, role := range roles {
		names = append(names, role.Name)
	}
	sort.Strings(names)
	return names
}

func HasPermission(names []string, target string) bool {
	for _, name := range names {
		if name == target {
			return true
		}
	}
	return false
}

func LegacyMembershipRole(roles []Role) MembershipRole {
	for _, role := range roles {
		if role.Type == RoleTypeOwner {
			return MembershipRoleOwner
		}
	}
	return MembershipRoleMember
}

func PermissionsMaskFromNames(names []string) (int64, error) {
	var mask int64
	seen := map[string]struct{}{}
	for _, rawName := range names {
		name := strings.TrimSpace(strings.ToLower(rawName))
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}

		matched := false
		for _, permission := range permissionRegistry {
			if permission.Name == name {
				mask |= permission.Bit
				matched = true
				break
			}
		}
		if !matched {
			return 0, ErrPermissionRegistryUnknown
		}
	}
	return mask, nil
}

func AllPermissionsMask() int64 {
	var mask int64
	for _, permission := range permissionRegistry {
		mask |= permission.Bit
	}
	return mask
}

func DefaultRoleDefinitions(now time.Time) []Role {
	return []Role{
		{
			Name:            string(MembershipRoleOwner),
			Type:            RoleTypeOwner,
			PermissionsMask: AllPermissionsMask(),
			Builtin:         true,
			Status:          RoleStatusActive,
			Version:         1,
			CreatedAt:       now,
			UpdatedAt:       now,
		},
		{
			Name:            string(MembershipRoleMember),
			Type:            RoleTypeMember,
			PermissionsMask: permissionMaskOrZero(PermissionWorkspaceRead),
			Builtin:         true,
			Status:          RoleStatusActive,
			Version:         1,
			CreatedAt:       now,
			UpdatedAt:       now,
		},
	}
}

func permissionMaskOrZero(names ...string) int64 {
	mask, err := PermissionsMaskFromNames(names)
	if err != nil {
		return 0
	}
	return mask
}
