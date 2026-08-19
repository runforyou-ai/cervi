import {
  GetUser,
  ListUsers,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/service"
import {
  UserRole,
  UserStatus,
  type DirectoryUser,
  type UserListInput,
  type UserList as UserListResponse,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/models"
import { call } from "@/api/client"
import { optionalWailsEnum } from "@/lib/wails-enum"

export { UserRole, UserStatus }
export type { DirectoryUser, UserListResponse }

export type UserListQuery = Omit<
  Partial<UserListInput>,
  "query"
> & {
  q?: string
}

/** 从查询参数解析成员状态，空值表示不筛选。 */
export function userStatusFromQuery(value: string | null) {
  return optionalWailsEnum(UserStatus, value)
}

/** 从查询参数解析成员角色，空值表示不筛选。 */
export function userRoleFromQuery(value: string | null) {
  return optionalWailsEnum(UserRole, value)
}

export async function listUsers(query: UserListQuery, signal?: AbortSignal) {
  const output = await call(
    (meta) =>
      ListUsers(meta, {
        query: query.q ?? "",
        status: query.status ?? UserStatus.$zero,
        role: query.role ?? UserRole.$zero,
        page: query.page ?? 1,
        pageSize: query.pageSize ?? 50,
      }),
    signal,
  )
  return { ...output, users: output.users ?? [] }
}

export function getUser(userId: string, signal?: AbortSignal) {
  return call((meta) => GetUser(meta, userId), signal)
}
