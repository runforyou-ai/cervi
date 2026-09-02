/** 当前会话的新消息通知策略与投递编排。 */
import {
  WorkStatus,
  type MessageNotificationInput,
} from "@/api"
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
  return document.visibilityState === "visible" && document.hasFocus()
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
