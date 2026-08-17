import { request } from "@/api/client"

export type WebsiteChannel = {
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

export type WebsiteChannelInput = {
  name: string
  description: string
  defaultLocale: "zh-CN" | "en-US"
}

export type ChannelSummary = {
  id: string
  type: string
  name: string
}

type WebsiteChannelListResponse = {
  channels: WebsiteChannel[]
}

type ChannelSummaryListResponse = {
  channels: ChannelSummary[]
}

export async function listChannels(signal?: AbortSignal) {
  const response = await request<ChannelSummaryListResponse>("/channels", {
    signal,
  })
  return response.channels
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
  return request<WebsiteChannel>("/channels/website", {
    method: "POST",
    body: JSON.stringify(input),
  })
}

export function updateWebsiteChannel(
  channelId: string,
  input: WebsiteChannelInput
) {
  return request<WebsiteChannel>(`/channels/website/${channelId}`, {
    method: "PATCH",
    body: JSON.stringify(input),
  })
}

export function deleteWebsiteChannel(channelId: string) {
  return request<void>(`/channels/website/${channelId}`, {
    method: "DELETE",
  })
}

export function restoreWebsiteChannel(channelId: string) {
  return request<WebsiteChannel>(`/channels/website/${channelId}/restore`, {
    method: "POST",
  })
}
