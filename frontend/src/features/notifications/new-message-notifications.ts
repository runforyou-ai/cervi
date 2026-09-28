/** 当前会话的新消息通知策略与投递编排。 */
import {
  WorkStatus,
  type MessageNotificationInput,
} from "@/api"
import { resolveAppPlatform } from "@/platform/app-platform"
import {
  canSendNotification,
  checkNotificationPermission,
  deliverMessageNotification,
  readNotificationDevicePreferences,
  type NotificationDeviceScope,
} from "@/platform/notifications"

type NewMessageNotificationOptions = Omit<
  MessageNotificationInput,
  "soundEnabled"
> & {
  scope: NotificationDeviceScope
}

type NotificationPolicy = {
  token: symbol
  scope: NotificationDeviceScope
  attentionEnabled: boolean
}

let activeNotificationPolicy: NotificationPolicy | null = null
let messageNotificationQueue: Promise<void> = Promise.resolve()

/** 判断当前会话策略是否仍允许发送通知。 */
function canDeliverWithPolicy(
  scope: NotificationDeviceScope,
  token: symbol,
) {
  return (
    activeNotificationPolicy?.attentionEnabled === true &&
    // 判断两个通知设备范围是否相同。
    activeNotificationPolicy.scope.organizationId === scope.organizationId &&
    activeNotificationPolicy.scope.userId === scope.userId &&
    activeNotificationPolicy.token === token
  )
}

/** 判断用户是否正在查看应用。 */
function isApplicationVisible() {
  if (document.visibilityState !== "visible") {
    return false
  }
  // 移动端 WebView 没有窗口焦点语义，应用在前台即视为正在查看。
  return resolveAppPlatform() === "mobile" || document.hasFocus()
}

/** 激活当前用户的新消息通知策略。 */
export function activateNotificationPolicy(
  scope: NotificationDeviceScope,
  messageNotificationsEnabled: boolean,
  workStatus: WorkStatus,
) {
  const token = Symbol("notification-policy")
  activeNotificationPolicy = {
    token,
    scope,
    attentionEnabled:
      messageNotificationsEnabled &&
      workStatus === WorkStatus.WorkStatusWorking,
  }

  return () => {
    if (activeNotificationPolicy?.token === token) {
      activeNotificationPolicy = null
    }
  }
}

/** 停止当前用户的新消息通知策略。 */
export function deactivateNotificationPolicy() {
  activeNotificationPolicy = null
}

/** 按到达顺序处理一条新消息通知。 */
export function notifyNewMessage(options: NewMessageNotificationOptions) {
  const policy = activeNotificationPolicy
  if (
    !policy ||
    !canDeliverWithPolicy(options.scope, policy.token) ||
    isApplicationVisible()
  ) {
    return Promise.resolve(false)
  }

  const delivery = messageNotificationQueue.then(async () => {
    if (
      !canDeliverWithPolicy(options.scope, policy.token) ||
      isApplicationVisible()
    ) {
      return false
    }

    const permission = await checkNotificationPermission()
    if (
      !canSendNotification(permission) ||
      !canDeliverWithPolicy(options.scope, policy.token) ||
      isApplicationVisible()
    ) {
      return false
    }

    const { soundEnabled } = readNotificationDevicePreferences(options.scope)
    await deliverMessageNotification({
      id: options.id,
      title: options.title,
      body: options.body,
      soundEnabled,
      path: options.path,
    })
    return true
  })
  messageNotificationQueue = delivery.then(
    () => undefined,
    (error) => {
      console.warn("处理新消息通知失败", {
        notification_id: options.id,
        error,
      })
    },
  )
  return delivery
}

/**
 * 按到达顺序处理一条其他工作区的新消息通知：按该工作区本人的提醒开关与工作状态判断，
 * 用户正停在另一个工作区、看不到这些消息，应用在前台时同样投递；当前没有登录中的工作区时不投递。
 */
export function notifyOtherWorkspaceMessage(
  options: NewMessageNotificationOptions,
  attentionEnabled: boolean,
) {
  if (!attentionEnabled || !activeNotificationPolicy) {
    return Promise.resolve(false)
  }
  const delivery = messageNotificationQueue.then(async () => {
    if (!activeNotificationPolicy) {
      return false
    }
    const permission = await checkNotificationPermission()
    if (!canSendNotification(permission)) {
      return false
    }
    const { soundEnabled } = readNotificationDevicePreferences(options.scope)
    await deliverMessageNotification({
      id: options.id,
      title: options.title,
      body: options.body,
      soundEnabled,
      path: options.path,
    })
    return true
  })
  messageNotificationQueue = delivery.then(
    () => undefined,
    (error) => {
      console.warn("处理其他工作区的新消息通知失败", {
        notification_id: options.id,
        error,
      })
    },
  )
  return delivery
}
