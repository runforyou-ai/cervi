/** 企业成员账号调用。 */
import {
  CreateUser,
  DeactivateUser,
  GetUser,
  ListUsers,
  ReactivateUser,
  UpdateUser,
  UpdateUserWorkStatus,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/service"
import type {
  User,
  UserListInput,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/models"
import { bind } from "@/api/client"
import type { NonNullArrays } from "@/api/normalize"

export type UserListQuery = Partial<UserListInput>

export type UserData = NonNullArrays<User>

const listUsersBound = bind(ListUsers)

/** 修改当前用户主动设置的工作状态。 */
export const updateUserWorkStatus = bind(UpdateUserWorkStatus)

/** 读取企业成员详情。 */
export const getUser = bind(GetUser)

/** 创建企业成员账号。 */
export const createUser = bind(CreateUser)

/** 修改企业成员资料、角色和所属团队。 */
export const updateUser = bind(UpdateUser)

/** 禁用企业成员账号。 */
export const deactivateUser = bind(DeactivateUser)

/** 将企业成员账号恢复为正常状态。 */
export const reactivateUser = bind(ReactivateUser)

/** 读取企业成员列表。 */
export function listUsers(query: UserListQuery, signal?: AbortSignal) {
  return listUsersBound(
    {
      query: query.query ?? "",
      status: query.status ?? null,
      roleId: query.roleId ?? "",
      teamId: query.teamId ?? "",
      page: query.page ?? 1,
      pageSize: query.pageSize ?? 50,
    },
    signal,
  )
}
