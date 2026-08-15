import { request } from "@/api/client"
import type { Principal } from "@/api/identity"

export function install(input: {
  organizationName: string
  displayName: string
  email: string
  password: string
}) {
  return request<Principal>("/install", {
    method: "POST",
    body: JSON.stringify(input),
  })
}
