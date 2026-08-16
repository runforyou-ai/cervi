import { request } from "@/api/client"
import type { Principal } from "@/api/identity"

export function login(input: { email: string; password: string }) {
  return request<Principal>("/auth/login", {
    method: "POST",
    body: JSON.stringify(input),
  })
}

export function logout() {
  return request<void>("/auth/logout", { method: "POST" })
}
