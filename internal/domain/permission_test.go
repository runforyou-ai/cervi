package domain

import "testing"

// TestPermissionDefinitionsExcludeInbox 验证权限目录只包含当前可配置功能且不定义收件箱权限。
func TestPermissionDefinitionsExcludeInbox(t *testing.T) {
	definitions := PermissionDefinitions()
	if len(definitions) != 12 {
		t.Fatalf("permission count = %d, want 12", len(definitions))
	}
	for _, definition := range definitions {
		if definition.Code == "" || definition.Resource == "" || definition.Level == "" {
			t.Fatalf("incomplete permission definition: %+v", definition)
		}
		if definition.Resource == "inbox" {
			t.Fatalf("inbox permission must not be predefined: %+v", definition)
		}
	}
}

// TestDefaultRolePermissions 验证三个内置角色的默认权限。
func TestDefaultRolePermissions(t *testing.T) {
	if got, want := len(DefaultRolePermissions(RoleKindAdmin)), len(PermissionDefinitions()); got != want {
		t.Fatalf("administrator permission count = %d, want %d", got, want)
	}
	if got := DefaultRolePermissions(RoleKindCustomerService); len(got) != 4 {
		t.Fatalf("customer service permissions = %v, want 4 items", got)
	}
	if got := DefaultRolePermissions(RoleKindMember); len(got) != 1 || got[0] != PermissionTeamMembersView {
		t.Fatalf("member permissions = %v, want team member view", got)
	}
}
