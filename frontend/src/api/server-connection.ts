import { request } from "@/api/client"

export function connectServer(serverUrl: string) {
  return request<void>("/server-connection", {
    method: "POST",
    body: JSON.stringify({ serverUrl }),
  })
}
