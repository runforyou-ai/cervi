/** 其他工作区的新消息与客服提醒系统通知：按工作区动态事件流为每个其他工作区维护新消息观察器，按该工作区本人的提醒设置投递，点击进入对应工作区的会话。 */
import { useEffect, useEffectEvent, useMemo } from "react"
import { useTranslation } from "react-i18next"

import {
  InboxScope,
  WorkStatus,
  callInWorkspace,
  getInboxConversation,
  isApiError,
  loadIdentity,
  loadInbox,
  readConversationAttention,
  workspaceActivityClient,
  type ConversationAttentionMessage,
  type Identity,
  type InboxConversationData,
  type Workspace,
} from "@/api"
import type { RealtimeServerFrame } from "@/api/realtime/protocol"
import { useConversationName } from "@/hooks/use-conversation-name"
import { workspaceHref } from "@/lib/workspace-route"
import { NewMessageWatcher } from "./new-message-watcher"
import { notifyOtherWorkspaceMessage } from "./new-message-notifications"
import { messageNotificationBody, serviceAttentionBodyKeys } from "./use-new-message-notifications"

type ServiceAttentionActivity = Extract<RealtimeServerFrame, { type: "workspace_activity" }> & { kind: "service_attention" }

/** 判断其他工作区的读取失败是否只因会话或成员身份已不可用，这类失败不影响当前工作区，按不可读处理。 */
function isUnavailable(error: unknown) {
  return isApiError(error) && (error.kind === "not_found" || error.state !== "")
}

/** 在工作区外壳内为当前工作区之外的全部工作区投递新消息与客服提醒通知；conversationPath 给出会话在工作区内的页面。 */
export function useOtherWorkspaceNotifications(
  currentWorkspaceId: string,
  workspaces: Workspace[],
  conversationPath: (conversation: InboxConversationData) => string,
) {
  const { t } = useTranslation("inbox")
  const conversationName = useConversationName()
  // 工作区列表按内容比较，重新读取得到的同一列表不重建观察器。
  const othersKey = workspaces
    .filter((workspace) => workspace.id !== currentWorkspaceId)
    .map((workspace) => `${workspace.id}:${workspace.slug}:${workspace.name}`)
    .join("\n")
  const others = useMemo(
    () =>
      othersKey
        ? othersKey.split("\n").map((line) => {
            const [id, slug, ...name] = line.split(":")
            return { id, slug, name: name.join(":") }
          })
        : [],
    [othersKey],
  )

  /** 投递一条其他工作区的通知，按该工作区本人的提醒开关与工作状态判断。 */
  const deliver = useEffectEvent(
    async (
      workspace: { id: string; slug: string; name: string },
      identity: Identity,
      id: string,
      conversation: InboxConversationData,
      body: string,
    ) => {
      await notifyOtherWorkspaceMessage(
        {
          id,
          title: t("notificationOtherWorkspaceTitle", { workspace: workspace.name, title: conversationName(conversation) }),
          body,
          path: workspaceHref(workspace.slug, conversationPath(conversation)),
          scope: { organizationId: identity.user.organizationId, userId: identity.user.id },
        },
        identity.user.messageNotificationsEnabled && identity.user.workStatus === WorkStatus.WorkStatusWorking,
      )
    },
  )
  const messageBody = useEffectEvent((conversation: InboxConversationData, message: ConversationAttentionMessage) =>
    messageNotificationBody(t, conversation, message),
  )
  const attentionBody = useEffectEvent((frame: ServiceAttentionActivity) =>
    frame.reason ? t(serviceAttentionBodyKeys[frame.reason]) : "",
  )

  useEffect(() => {
    if (others.length === 0) return
    const byId = new Map(others.map((workspace) => [workspace.id, workspace]))
    const watchers = new Map<string, NewMessageWatcher>()
    // 各工作区本人的身份与提醒设置，资料变化或重新连接后重新读取。
    const identities = new Map<string, Promise<Identity>>()

    /** 读取本人在指定工作区的身份，失败时下次重新读取。 */
    const readIdentity = (workspaceId: string) => {
      let identity = identities.get(workspaceId)
      if (!identity) {
        identity = callInWorkspace(workspaceId, () => loadIdentity())
        identity.catch(() => identities.delete(workspaceId))
        identities.set(workspaceId, identity)
      }
      return identity
    }

    /** 返回指定工作区的新消息观察器，首次使用时创建；读取都以该工作区为目标。 */
    const watcherFor = (workspace: { id: string; slug: string; name: string }) => {
      let watcher = watchers.get(workspace.id)
      if (watcher) return watcher
      watcher = new NewMessageWatcher({
        // 基线覆盖本人在该工作区参与的聊天与待处理的服务会话。
        readConversations: async () => {
          const [chats, pending] = await Promise.all([
            callInWorkspace(workspace.id, () => loadInbox({ scope: InboxScope.InboxScopeChat })),
            callInWorkspace(workspace.id, () => loadInbox({ scope: InboxScope.InboxScopePending })),
          ])
          return [...chats.conversations, ...pending.conversations]
        },
        readAttention: async (conversationId, afterMessageId) => {
          try {
            return await callInWorkspace(workspace.id, () => readConversationAttention(conversationId, { afterMessageId }))
          } catch (error) {
            if (isUnavailable(error)) return null
            throw error
          }
        },
        deliver: async (conversation, message) => {
          const identity = await readIdentity(workspace.id)
          await deliver(workspace, identity, message.id, conversation, messageBody(conversation, message))
        },
        failed: (error) => {
          if (!isUnavailable(error)) console.warn("处理其他工作区的新消息通知失败", { workspace_id: workspace.id, error })
        },
      })
      watchers.set(workspace.id, watcher)
      return watcher
    }

    /** 投递其他工作区的客服提醒，会话已不可读时结束。 */
    const deliverAttention = async (workspace: { id: string; slug: string; name: string }, frame: ServiceAttentionActivity) => {
      if (!frame.conversationId) return
      const conversationId = frame.conversationId
      let conversation: InboxConversationData
      try {
        conversation = await callInWorkspace(workspace.id, () => getInboxConversation(conversationId))
      } catch (error) {
        if (isUnavailable(error)) return
        throw error
      }
      const identity = await readIdentity(workspace.id)
      // 服务端每次提醒对应一次新的分配或等待轮次，通知编号按到达时间区分。
      await deliver(workspace, identity, `service_attention:${frame.serviceSessionId}:${frame.reason}:${Date.now()}`, conversation, attentionBody(frame))
    }

    const unsubscribe = workspaceActivityClient.subscribe((event) => {
      if (event.type !== "frame") return
      const frame = event.frame
      // 重新连接后各工作区重新取得基线，期间的消息只计入未读。
      if (frame.type === "server_hello") {
        identities.clear()
        for (const workspace of others) watcherFor(workspace).receive(frame)
        return
      }
      if (frame.type !== "workspace_activity") return
      const workspace = byId.get(frame.workspaceId)
      if (!workspace) return
      switch (frame.kind) {
        case "conversation_changed":
          // 只有时间线变化可能带来新消息；变化类别未知时按全部类别处理。
          if (frame.conversationId && (!frame.changes || frame.changes.includes("timeline"))) {
            watcherFor(workspace).receive({ type: "conversation_changed", conversationId: frame.conversationId, conversationType: "group", version: 0n, changes: frame.changes })
          }
          return
        case "conversation_removed":
          if (frame.conversationId) watcherFor(workspace).receive({ type: "conversation_removed", conversationId: frame.conversationId })
          return
        case "service_attention":
          void deliverAttention(workspace, frame as ServiceAttentionActivity).catch((error: unknown) => {
            console.warn("处理其他工作区的客服提醒通知失败", { workspace_id: workspace.id, error })
          })
          return
        case "identity_profile_changed":
          identities.delete(workspace.id)
          return
      }
    })
    // 订阅时事件流可能已经建立，此时不会再收到问候事件，直接为各工作区取一次基线。
    if (workspaceActivityClient.state === "ready") {
      for (const workspace of others) watcherFor(workspace).start()
    }
    return () => {
      unsubscribe()
      for (const watcher of watchers.values()) watcher.dispose()
    }
  }, [others])
}
