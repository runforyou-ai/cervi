import { request } from "@/api/client"
import type { Principal } from "@/api/identity"

export type Conversation = {
  id: string
  name: string
  initials: string
  channel: string
  preview: string
  time: string
  status: string
  unread?: number
  online?: boolean
  messages: {
    id: string
    author: "visitor" | "agent"
    text: string
    time: string
  }[]
}

export type InboxData = Principal & {
  conversations: Conversation[]
}

export function loadInbox() {
  return request<InboxData>("/inbox")
}
