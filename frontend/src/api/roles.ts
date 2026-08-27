/** 角色与权限调用与归一化。 */
import {
  CreateRole,
  DeleteRole,
  GetRole,
  ListRoles,
  UpdateRole,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/service"
import type {
  PermissionDefinition,
  Role,
  RoleInput,
  RoleList,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/models"
import { bind } from "@/api/client"
import { asList } from "@/api/normalize"

export type RoleData = Omit<Role, "permissions"> & {
  permissions: NonNullable<Role["permissions"]>
}

export type RoleListData = Omit<RoleList, "roles" | "permissions"> & {
  roles: RoleData[]
  permissions: PermissionDefinition[]
}

const listRolesBound = bind(ListRoles)
const getRoleBound = bind(GetRole)
const createRoleBound = bind(CreateRole)
const updateRoleBound = bind(UpdateRole)

/** 删除自定义角色。 */
export const deleteRole = bind(DeleteRole)

/** 读取角色、数量上限和权限目录。 */
export function listRoles(signal?: AbortSignal): Promise<RoleListData> {
  return listRolesBound(signal).then((output) => ({
    ...output,
    roles: asList(output.roles).map(normalizeRole),
    permissions: asList(output.permissions),
  }))
}

/** 读取角色详情。 */
export function getRole(roleId: string, signal?: AbortSignal) {
  return getRoleBound(roleId, signal).then(normalizeRole)
}

/** 创建自定义角色。 */
export function createRole(input: RoleInput) {
  return createRoleBound(input).then(normalizeRole)
}

/** 修改角色信息和权限。 */
export function updateRole(roleId: string, input: RoleInput) {
  return updateRoleBound(roleId, input).then(normalizeRole)
}

/** 把角色中的可空权限切片转换为空数组。 */
function normalizeRole(role: Role): RoleData {
  return { ...role, permissions: asList(role.permissions) }
}
