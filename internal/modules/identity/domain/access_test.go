package domain

import (
	"testing"
	"time"
)

func TestNewPermissionsInRegistry(t *testing.T) {
	registry := PermissionRegistry()
	permNames := make(map[string]bool)
	for _, p := range registry {
		permNames[p.Name] = true
	}

	expected := []string{
		PermissionWorkspaceRead,
		PermissionWorkspaceManageMembers,
		PermissionWorkspaceManageRoles,
		PermissionWorkspaceManageSetting,
		PermissionSenderManage,
		PermissionAudienceRead,
		PermissionAudienceWrite,
		PermissionAudienceImport,
		PermissionAudienceExport,
		PermissionTemplateRead,
		PermissionTemplateWrite,
		PermissionTemplateRender,
		PermissionSuppressionRead,
		PermissionSuppressionManage,
		PermissionCampaignRead,
		PermissionCampaignWrite,
		PermissionCampaignSend,
		PermissionAPIKeyManage,
	}

	for _, name := range expected {
		if !permNames[name] {
			t.Errorf("expected permission %q to be in registry", name)
		}
	}
}

func TestAllPermissionsMaskIncludesNewPermissions(t *testing.T) {
	mask := AllPermissionsMask()

	allPerms := []string{
		PermissionWorkspaceRead,
		PermissionWorkspaceManageMembers,
		PermissionWorkspaceManageRoles,
		PermissionWorkspaceManageSetting,
		PermissionSenderManage,
		PermissionAudienceRead,
		PermissionAudienceWrite,
		PermissionAudienceImport,
		PermissionAudienceExport,
		PermissionTemplateRead,
		PermissionTemplateWrite,
		PermissionTemplateRender,
		PermissionSuppressionRead,
		PermissionSuppressionManage,
		PermissionCampaignRead,
		PermissionCampaignWrite,
		PermissionCampaignSend,
		PermissionAPIKeyManage,
	}

	names := PermissionNamesFromMask(mask)
	nameSet := make(map[string]bool)
	for _, n := range names {
		nameSet[n] = true
	}

	for _, name := range allPerms {
		if !nameSet[name] {
			t.Errorf("AllPermissionsMask should include %q", name)
		}
	}
}

func TestPermissionsMaskFromNamesAcceptsNewPermissions(t *testing.T) {
	newPerms := []string{
		PermissionAudienceRead,
		PermissionAudienceWrite,
		PermissionAudienceImport,
		PermissionAudienceExport,
		PermissionTemplateRead,
		PermissionTemplateWrite,
		PermissionTemplateRender,
		PermissionSuppressionRead,
		PermissionSuppressionManage,
		PermissionCampaignRead,
		PermissionCampaignWrite,
		PermissionCampaignSend,
		PermissionAPIKeyManage,
	}

	for _, name := range newPerms {
		mask, err := PermissionsMaskFromNames([]string{name})
		if err != nil {
			t.Errorf("PermissionsMaskFromNames should accept %q, got error: %v", name, err)
		}
		if mask == 0 {
			t.Errorf("PermissionsMaskFromNames(%q) should return non-zero mask", name)
		}
	}
}

func TestPermissionsMaskFromNamesUnknownReturnsError(t *testing.T) {
	_, err := PermissionsMaskFromNames([]string{"nonexistent.permission"})
	if err != ErrPermissionRegistryUnknown {
		t.Errorf("expected ErrPermissionRegistryUnknown, got %v", err)
	}
}

func TestOwnerMaskIncludesNewPermissions(t *testing.T) {
	roles := DefaultRoleDefinitions(time.Now())
	var owner Role
	for _, r := range roles {
		if r.Type == RoleTypeOwner {
			owner = r
			break
		}
	}

	perms := PermissionNamesFromMask(owner.PermissionsMask)
	permSet := make(map[string]bool)
	for _, p := range perms {
		permSet[p] = true
	}

	newPerms := []string{
		PermissionAudienceRead,
		PermissionAudienceWrite,
		PermissionAudienceImport,
		PermissionAudienceExport,
		PermissionTemplateRead,
		PermissionTemplateWrite,
		PermissionTemplateRender,
		PermissionSuppressionRead,
		PermissionSuppressionManage,
		PermissionCampaignRead,
		PermissionCampaignWrite,
		PermissionCampaignSend,
		PermissionAPIKeyManage,
	}

	for _, name := range newPerms {
		if !permSet[name] {
			t.Errorf("owner role should include %q", name)
		}
	}
}

func TestMemberRoleHasOnlyWorkspaceRead(t *testing.T) {
	roles := DefaultRoleDefinitions(time.Now())
	var member Role
	for _, r := range roles {
		if r.Type == RoleTypeMember {
			member = r
			break
		}
	}

	if member.PermissionsMask == 0 {
		t.Fatal("member role should have non-zero permissions mask")
	}

	perms := PermissionNamesFromMask(member.PermissionsMask)
	if len(perms) != 1 || perms[0] != PermissionWorkspaceRead {
		t.Errorf("member role should only have %q, got %v", PermissionWorkspaceRead, perms)
	}
}
