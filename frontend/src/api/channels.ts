import {
  CreateWebsiteChannel,
  DeleteWebsiteChannel,
  GetWebsiteChannel,
  ListChannels,
  ListWebsiteChannels,
  RestoreWebsiteChannel,
  UpdateWebsiteChannel,
  UpdateWebsiteChannelChatInterface,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/service"
import type {
  WebsiteChannelChatInterfaceInput,
  WebsiteChannelInput,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/models"
import { call } from "@/api/client"

export { ChannelType, Locale } from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/models"
export type {
  ChannelSummary,
  WebsiteChannel,
  WebsiteChannelChatInterface,
  WebsiteChannelChatInterfaceInput,
  WebsiteChannelInput,
  WebsiteChannelSummary,
} from "../../bindings/github.com/runforyou-ai/cervi/internal/appservice/models"

export async function listChannels(signal?: AbortSignal) {
  return (await call((meta) => ListChannels(meta), signal)) ?? []
}

export async function listWebsiteChannels() {
  return (await call((meta) => ListWebsiteChannels(meta, false))) ?? []
}

export async function listDeletedWebsiteChannels() {
  return (await call((meta) => ListWebsiteChannels(meta, true))) ?? []
}

export function getWebsiteChannel(channelId: string) {
  return call((meta) => GetWebsiteChannel(meta, channelId))
}

export function createWebsiteChannel(input: WebsiteChannelInput) {
  return call((meta) => CreateWebsiteChannel(meta, input))
}

export function updateWebsiteChannel(
  channelId: string,
  input: WebsiteChannelInput,
) {
  return call((meta) => UpdateWebsiteChannel(meta, channelId, input))
}

export function updateWebsiteChannelChatInterface(
  channelId: string,
  input: WebsiteChannelChatInterfaceInput,
) {
  return call((meta) =>
    UpdateWebsiteChannelChatInterface(meta, channelId, input),
  )
}

export function deleteWebsiteChannel(channelId: string) {
  return call((meta) => DeleteWebsiteChannel(meta, channelId))
}

export function restoreWebsiteChannel(channelId: string) {
  return call((meta) => RestoreWebsiteChannel(meta, channelId))
}
