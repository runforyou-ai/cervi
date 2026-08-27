/** 企业成员账号调用与归一化。 */
import {
  CreateUser,
  DeactivateUser,
  GetUser,
  ListUsers,
  ReactivateUser,
  UpdateUser,
  UpdateUserRoles,
  UpdateUserWorkStatus,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/service"
import type {
  CreateUserInput,
  UpdateUserInput,
  User,
  UserList,
  UserListInput,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/models"
import { bind } from "@/api/client"
import { asList } from "@/api/normalize"

export type UserListQuery = Partial<UserListInput>

export type UserData = Omit<User, "teams"> & {
  teams: NonNullable<User["teams"]>
}

export type UserListResponse = Omit<UserList, "users"> & {
  users: UserData[]
}

const listUsersBound = bind(ListUsers)
const getUserBound = bind(GetUser)
const createUserBound = bind(CreateUser)
const updateUserBound = bind(UpdateUser)
const deactivateUserBound = bind(DeactivateUser)
const reactivateUserBound = bind(ReactivateUser)

/** 修改当前用户主动设置的工作状态。 */
export const updateUserWorkStatus = bind(UpdateUserWorkStatus)

/** 在一个事务中批量调整企业成员角色。 */
export const updateUserRoles = bind(UpdateUserRoles)

/** 读取企业成员详情。 */
export function getUser(userId: string, signal?: AbortSignal) {
  return getUserBound(userId, signal).then(normalizeUser)
}

/** 创建企业成员账号。 */
export function createUser(input: CreateUserInput) {
  return createUserBound(input).then(normalizeUser)
}

/** 修改企业成员资料、角色和所属团队。 */
export function updateUser(userId: string, input: UpdateUserInput) {
  return updateUserBound(userId, input).then(normalizeUser)
}

/** 禁用企业成员账号。 */
export function deactivateUser(userId: string) {
  return deactivateUserBound(userId).then(normalizeUser)
}

/** 将企业成员账号恢复为正常状态。 */
export function reactivateUser(userId: string) {
  return reactivateUserBound(userId).then(normalizeUser)
}

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
  ).then((output) => ({
    ...output,
    users: asList(output.users).map(normalizeUser),
  }))
}

/** 归一化企业成员所属团队。 */
function normalizeUser(user: User): UserData {
  return { ...user, teams: asList(user.teams) }
}
