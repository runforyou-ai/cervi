package domain

// PermissionCode 定义可分配给角色的权限。
type PermissionCode string

const (
	PermissionExternalContactsView   PermissionCode = "external_contacts.view"
	PermissionExternalContactsManage PermissionCode = "external_contacts.manage"
	PermissionTeamMembersView        PermissionCode = "team_members.view"
	PermissionTeamMembersManage      PermissionCode = "team_members.manage"
	PermissionChannelsView           PermissionCode = "channels.view"
	PermissionChannelsManage         PermissionCode = "channels.manage"
	PermissionRolesView              PermissionCode = "roles.view"
	PermissionRolesManage            PermissionCode = "roles.manage"
	PermissionOrganizationView       PermissionCode = "organization.view"
	PermissionOrganizationManage     PermissionCode = "organization.manage"
)

// PermissionResource 定义权限所属功能。
type PermissionResource string

const (
	PermissionResourceExternalContacts PermissionResource = "external_contacts"
	PermissionResourceTeamMembers      PermissionResource = "team_members"
	PermissionResourceChannels         PermissionResource = "channels"
	PermissionResourceRoles            PermissionResource = "roles"
	PermissionResourceOrganization     PermissionResource = "organization"
)

// PermissionLevel 定义权限操作层级。
type PermissionLevel string

const (
	PermissionLevelView   PermissionLevel = "view"
	PermissionLevelManage PermissionLevel = "manage"
)

// PermissionAppliesTo 定义权限适用的企业身份类型：管理台操作只对成员生效，查看类权限同时作为 AI 员工只读工具的门槛。
type PermissionAppliesTo string

const (
	PermissionAppliesToMember PermissionAppliesTo = "member"
	PermissionAppliesToAgent  PermissionAppliesTo = "agent"
	PermissionAppliesToBoth   PermissionAppliesTo = "both"
)

// PermissionDefinition 描述一项预定义权限。
type PermissionDefinition struct {
	Code      PermissionCode
	Resource  PermissionResource
	Level     PermissionLevel
	AppliesTo PermissionAppliesTo
}

var permissionDefinitions = []PermissionDefinition{
	{Code: PermissionExternalContactsView, Resource: PermissionResourceExternalContacts, Level: PermissionLevelView, AppliesTo: PermissionAppliesToBoth},
	{Code: PermissionExternalContactsManage, Resource: PermissionResourceExternalContacts, Level: PermissionLevelManage, AppliesTo: PermissionAppliesToMember},
	{Code: PermissionTeamMembersView, Resource: PermissionResourceTeamMembers, Level: PermissionLevelView, AppliesTo: PermissionAppliesToBoth},
	{Code: PermissionTeamMembersManage, Resource: PermissionResourceTeamMembers, Level: PermissionLevelManage, AppliesTo: PermissionAppliesToMember},
	{Code: PermissionChannelsView, Resource: PermissionResourceChannels, Level: PermissionLevelView, AppliesTo: PermissionAppliesToBoth},
	{Code: PermissionChannelsManage, Resource: PermissionResourceChannels, Level: PermissionLevelManage, AppliesTo: PermissionAppliesToMember},
	{Code: PermissionRolesView, Resource: PermissionResourceRoles, Level: PermissionLevelView, AppliesTo: PermissionAppliesToMember},
	{Code: PermissionRolesManage, Resource: PermissionResourceRoles, Level: PermissionLevelManage, AppliesTo: PermissionAppliesToMember},
	{Code: PermissionOrganizationView, Resource: PermissionResourceOrganization, Level: PermissionLevelView, AppliesTo: PermissionAppliesToMember},
	{Code: PermissionOrganizationManage, Resource: PermissionResourceOrganization, Level: PermissionLevelManage, AppliesTo: PermissionAppliesToMember},
}

// PermissionDefinitions 返回按界面顺序排列的权限目录。
func PermissionDefinitions() []PermissionDefinition {
	return append([]PermissionDefinition(nil), permissionDefinitions...)
}

// IsPermissionCode 判断权限代码是否受支持。
func IsPermissionCode(code PermissionCode) bool {
	for _, definition := range permissionDefinitions {
		if definition.Code == code {
			return true
		}
	}
	return false
}

// PermissionViewDependency 返回管理权限依赖的查看权限。
func PermissionViewDependency(code PermissionCode) (PermissionCode, bool) {
	for _, definition := range permissionDefinitions {
		if definition.Code != code || definition.Level != PermissionLevelManage {
			continue
		}
		for _, candidate := range permissionDefinitions {
			if candidate.Resource == definition.Resource && candidate.Level == PermissionLevelView {
				return candidate.Code, true
			}
		}
	}
	return "", false
}
