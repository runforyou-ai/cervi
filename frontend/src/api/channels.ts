import { request } from "@/api/client"

export type WebsiteChannelSummary = {
  id: string
  organizationId: string
  createdByUserId: string
  type: "website"
  name: string
  description: string | null
  defaultLocale: "zh-CN" | "en-US"
  createdAt: string
  updatedAt: string
  deletedAt: string | null
}

export type WebsiteChannel = WebsiteChannelSummary & {
  chatInterface: WebsiteChannelChatInterface
}

export type WebsiteChannelChatInterface = {
  title: string
  subtitle: string | null
  greetingMessage: string | null
  themeColor: string
}

export type WebsiteChannelChatInterfaceInput = {
  title: string
  subtitle: string
  greetingMessage: string
  themeColor: string
}

export type WebsiteChannelInput = {
  name: string
  description: string
  defaultLocale: "zh-CN" | "en-US"
}

type WebsiteChannelListResponse = {
  channels: WebsiteChannelSummary[]
}

export async function listWebsiteChannels() {
  const response = await request<WebsiteChannelListResponse>("/channels/website")
  return response.channels
}

export async function listDeletedWebsiteChannels() {
  const response = await request<WebsiteChannelListResponse>(
    "/channels/website/trash"
  )
  return response.channels
}

export function getWebsiteChannel(channelId: string) {
  return request<WebsiteChannel>(`/channels/website/${channelId}`)
}

export function createWebsiteChannel(input: WebsiteChannelInput) {
  return request<WebsiteChannelSummary>("/channels/website", {
    method: "POST",
    body: JSON.stringify(input),
  })
}

export function updateWebsiteChannel(
  channelId: string,
  input: WebsiteChannelInput
) {
  return request<WebsiteChannelSummary>(`/channels/website/${channelId}`, {
    method: "PATCH",
    body: JSON.stringify(input),
  })
}

export function updateWebsiteChannelChatInterface(
  channelId: string,
  input: WebsiteChannelChatInterfaceInput
) {
  return request<WebsiteChannelChatInterface>(
    `/channels/website/${channelId}/chat-interface`,
    {
      method: "PATCH",
      body: JSON.stringify(input),
    }
  )
}

export function deleteWebsiteChannel(channelId: string) {
  return request<void>(`/channels/website/${channelId}`, {
    method: "DELETE",
  })
}

export function restoreWebsiteChannel(channelId: string) {
  return request<WebsiteChannelSummary>(
    `/channels/website/${channelId}/restore`,
    { method: "POST" }
  )
}
