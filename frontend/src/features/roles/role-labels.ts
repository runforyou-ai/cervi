/** 角色与权限的共享界面文案映射。 */
import type { TFunction } from "i18next"

import {
  PermissionResource,
  RoleKind,
  type RoleData,
} from "@/api"

/** 返回角色显示名称。 */
export function roleDisplayName(
  role: Pick<RoleData, "kind" | "name">,
  t: TFunction<"common">,
) {
  switch (role.kind) {
    case RoleKind.RoleKindAdmin:
      return t("roles.admin")
    case RoleKind.RoleKindCustomerService:
      return t("roles.customerService")
    case RoleKind.RoleKindMember:
      return t("roles.member")
    case RoleKind.RoleKindCustom:
      return role.name
    default:
      console.warn("未知的角色类型", role.kind)
      return role.name
  }
}

/** 返回内置角色说明。 */
export function builtInRoleDescription(
  role: RoleData,
  t: TFunction<"settings">,
) {
  switch (role.kind) {
    case RoleKind.RoleKindAdmin:
      return t("roles.kindsDescriptions.admin")
    case RoleKind.RoleKindCustomerService:
      return t("roles.kindsDescriptions.customerService")
    case RoleKind.RoleKindMember:
      return t("roles.kindsDescriptions.member")
    default:
      console.warn("非内置角色缺少固定说明", role.kind)
      return role.description
  }
}

/** 返回权限功能名称。 */
export function permissionResourceLabel(
  resource: PermissionResource,
  t: TFunction<"settings">,
) {
  switch (resource) {
    case PermissionResource.PermissionResourceExternalContacts:
      return t("roles.permissions.resources.externalContacts")
    case PermissionResource.PermissionResourceTeamMembers:
      return t("roles.permissions.resources.teamMembers")
    case PermissionResource.PermissionResourceChannels:
      return t("roles.permissions.resources.channels")
    case PermissionResource.PermissionResourceRoles:
      return t("roles.permissions.resources.roles")
    case PermissionResource.PermissionResourceOrganization:
      return t("roles.permissions.resources.organization")
    case PermissionResource.PermissionResourceStorage:
      return t("roles.permissions.resources.storage")
    default:
      console.warn("未知的权限功能", resource)
      return String(resource)
  }
}
