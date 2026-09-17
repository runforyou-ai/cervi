/** 移动端新消息系统通知与应用角标。 */
import { useEffect, useLayoutEffect } from "react"

import { loadInbox, WorkStatus, type Identity } from "@/api"
import { activateNotificationPolicy } from "@/features/notifications/new-message-notifications"
import { useNewMessageNotifications } from "@/features/notifications/use-new-message-notifications"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"
import { updateNotificationUnreadIndicator } from "@/platform/notifications"

/** 按当前身份投递新消息通知，并把提醒总数同步到应用角标。 */
export function useMobileMessageNotifications(identity: Identity | null) {
  const organizationId = identity?.organization.id
  const userId = identity?.user.id
  const notificationOrganizationId = identity?.user.organizationId
  const messageNotificationsEnabled = identity?.user.messageNotificationsEnabled
  const workStatus = identity?.user.workStatus

  /** 同步当前用户的新消息通知策略。 */
  useLayoutEffect(() => {
    if (
      !notificationOrganizationId ||
      !userId ||
      messageNotificationsEnabled === undefined ||
      workStatus === undefined
    ) {
      return
    }
    return activateNotificationPolicy(
      { organizationId: notificationOrganizationId, userId },
      messageNotificationsEnabled,
      workStatus,
    )
  }, [
    notificationOrganizationId,
    userId,
    messageNotificationsEnabled,
    workStatus,
  ])

  useNewMessageNotifications(identity, () => {})

  // 提醒总数按权威查询读取，与消息页签角标共用同一份缓存。
  const attention = useResource(
    resourceKeys.inboxAttention({
      organizationId: organizationId ?? "",
      userId: userId ?? "",
    }),
    async () => (await loadInbox({ limit: 1 })).attentionUnreadCount,
    { enabled: Boolean(organizationId && userId) },
  )
  const unreadCount = attention.data
  const attentionEnabled =
    Boolean(messageNotificationsEnabled) &&
    workStatus === WorkStatus.WorkStatusWorking

  /** 同步应用图标角标，提醒总数读到之前不改动角标。 */
  useEffect(() => {
    if (!userId || unreadCount === undefined) {
      return
    }
    void updateNotificationUnreadIndicator({
      count: unreadCount,
      attentionEnabled,
      attentionPending: false,
    }).catch((error) => {
      console.warn("同步移动端未读角标失败", { count: unreadCount, error })
    })
  }, [userId, unreadCount, attentionEnabled])

  /** 离开登录工作区时清除应用角标。 */
  useEffect(() => {
    return () => {
      void updateNotificationUnreadIndicator({
        count: 0,
        attentionEnabled: false,
        attentionPending: false,
      }).catch((error) => {
        console.warn("清除移动端未读角标失败", error)
      })
    }
  }, [])
}
