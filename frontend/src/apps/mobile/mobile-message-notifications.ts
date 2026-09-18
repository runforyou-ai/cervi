/** 移动端新消息系统通知与应用角标。 */
import { useEffect, useLayoutEffect } from "react"

import {
  loadInbox,
  NotificationPermissionStatus,
  WorkStatus,
  type Identity,
} from "@/api"
import { activateNotificationPolicy } from "@/features/notifications/new-message-notifications"
import { useNewMessageNotifications } from "@/features/notifications/use-new-message-notifications"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"
import {
  checkNotificationPermission,
  markNotificationPermissionRequested,
  readNotificationDevicePreferences,
  requestNotificationPermission,
  updateNotificationUnreadIndicator,
} from "@/platform/notifications"

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

  /** 登录后为本设备自动申请一次系统通知授权，之后交给偏好设置。 */
  useEffect(() => {
    if (!notificationOrganizationId || !userId) {
      return
    }
    const scope = { organizationId: notificationOrganizationId, userId }
    if (readNotificationDevicePreferences(scope).permissionAutoRequested) {
      return
    }
    void (async () => {
      const status = await checkNotificationPermission()
      // 仅在系统尚未记录选择时申请，已授权或已拒绝都不再打扰。
      if (
        status !==
        NotificationPermissionStatus.NotificationPermissionStatusPrompt
      ) {
        return
      }
      markNotificationPermissionRequested(scope)
      await requestNotificationPermission()
    })().catch((error) => {
      console.warn("申请移动端通知权限失败", error)
    })
  }, [notificationOrganizationId, userId])

  useNewMessageNotifications(identity, () => {})

  // 提醒总数按权威查询读取，与消息页签角标共用同一份缓存。
  const attention = useResource(
    resourceKeys.inboxAttention({
      organizationId: organizationId ?? "",
      userId: userId ?? "",
    }),
    async () => {
      // 应用角标合计内部会话提醒与客户会话中提醒本人的未读。
      const inbox = await loadInbox({ limit: 1 })
      return inbox.attentionUnreadCount + inbox.customerMentionedUnreadCount
    },
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
