import {
  GetUser,
  ListUsers,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/service"
import type {
  DirectoryUser,
  UserListInput,
  UserList as UserListResponse,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/models"
import {
  UserRole,
  UserStatus,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/domain/models"
import { call } from "@/api/client"

export { UserRole, UserStatus }
export type { DirectoryUser, UserListResponse }

export type UserListQuery = Omit<
  Partial<UserListInput>,
  "query"
> & {
  q?: string
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
