import { request } from "@/api/client"

export type DirectoryUser = {
  id: string
  organizationId: string
  email: string
  displayName: string
  role: string
  status: string
  createdAt: string
}

export type PageInfo = {
  number: number
  size: number
  total: number
}

type UserListResponse = {
  users: DirectoryUser[]
  page: PageInfo
}

export type UserListQuery = {
  q?: string
  status?: string
  role?: string
  page?: number
  pageSize?: number
}

export function listUsers(query: UserListQuery, signal?: AbortSignal) {
  const search = new URLSearchParams()
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined && value !== "") {
      search.set(key, String(value))
    }
  }
  return request<UserListResponse>(`/users?${search.toString()}`, { signal })
}

export function getUser(userId: string, signal?: AbortSignal) {
  return request<DirectoryUser>(`/users/${userId}`, { signal })
}
