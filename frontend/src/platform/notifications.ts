/** Web 与桌面端通知权限和设备偏好的统一适配。 */
import {
  checkNotificationPermission as checkDesktopNotificationPermission,
  requestNotificationPermission as requestDesktopNotificationPermission,
  sendNativeMessageNotification,
  updateUnreadIndicator as updateDesktopUnreadIndicator,
  WorkStatus,
  type MessageNotificationInput,
  type UnreadIndicatorState,
} from "@/api"
import {
  isDesktopMacOS,
  resolveAppPlatform,
} from "@/platform/app-platform"
import { openExternalURL } from "@/platform/open-external-url"

const notificationPreferencesChangedEvent =
  "cervi:notification-device-preferences-changed"
const notificationPreferencesStoragePrefix = "cervi.notifications"
const macOSNotificationSettingsURL =
  "x-apple.systempreferences:com.apple.preference.notifications"
let unreadIndicatorQueue: Promise<void> = Promise.resolve()
let messageNotificationQueue: Promise<void> = Promise.resolve()
let notificationRuntimePolicyGeneration = 0

export type NotificationPermissionState =
  | "prompt"
  | "granted"
  | "denied"
  | "system-managed"
  | "unsupported"

export type NotificationDeviceScope = {
  organizationId: string
  userId: string
}

export type NotificationDevicePreferences = {
  soundEnabled: boolean
  permissionRequested: boolean
  permissionMenuClickedOn: string
}

type StoredNotificationDevicePreferences = Partial<
  NotificationDevicePreferences
>

export type NewMessageNotificationOptions = Omit<
  MessageNotificationInput,
  "soundEnabled"
> & {
  scope: NotificationDeviceScope
}

type NotificationRuntimePolicy = {
  generation: number
  scopeKey: string
  messageNotificationsEnabled: boolean
  workStatus: WorkStatus
}

let activeNotificationRuntimePolicy: NotificationRuntimePolicy | null = null

const defaultNotificationDevicePreferences: NotificationDevicePreferences = {
  soundEnabled: true,
  permissionRequested: false,
  permissionMenuClickedOn: "",
}

/** 返回当前企业用户对应的本机通知偏好存储键。 */
function notificationPreferencesStorageKey(scope: NotificationDeviceScope) {
  return `${notificationPreferencesStoragePrefix}:${scope.organizationId}:${scope.userId}`
}

/** 激活当前会话的通知策略，并在策略或会话变化时使旧队列任务失效。 */
export function activateNotificationRuntimePolicy(
  scope: NotificationDeviceScope,
  messageNotificationsEnabled: boolean,
  workStatus: WorkStatus,
) {
  const generation = ++notificationRuntimePolicyGeneration
  activeNotificationRuntimePolicy = {
    generation,
    scopeKey: notificationPreferencesStorageKey(scope),
    messageNotificationsEnabled,
    workStatus,
  }

  return () => {
    if (activeNotificationRuntimePolicy?.generation !== generation) {
      return
    }
    notificationRuntimePolicyGeneration += 1
    activeNotificationRuntimePolicy = null
  }
}

/** 立即结束当前会话的通知策略，使尚未投递的旧消息失效。 */
export function deactivateNotificationRuntimePolicy() {
  notificationRuntimePolicyGeneration += 1
  activeNotificationRuntimePolicy = null
}

/** 返回仍属于当前会话和策略版本的通知策略。 */
function currentNotificationRuntimePolicy(
  scope: NotificationDeviceScope,
  generation?: number,
) {
  const policy = activeNotificationRuntimePolicy
  if (
    !policy ||
    policy.scopeKey !== notificationPreferencesStorageKey(scope) ||
    (generation !== undefined && policy.generation !== generation)
  ) {
    return null
  }
  return policy
}

/** 判断当前账号开关和工作状态是否允许主动提醒。 */
function notificationRuntimePolicyAllowsAttention(
  policy: NotificationRuntimePolicy | null,
) {
  return (
    policy?.messageNotificationsEnabled === true &&
    policy.workStatus === WorkStatus.WorkStatusWorking
  )
}

/** 判断用户是否正在查看应用，此时不重复发送系统提醒。 */
function isApplicationVisible() {
  return document.visibilityState === "visible" && document.hasFocus()
}

/** 读取本机保存的通知偏好，缺失或损坏时使用默认值。 */
export function readNotificationDevicePreferences(
  scope: NotificationDeviceScope,
): NotificationDevicePreferences {
  try {
    const stored = window.localStorage.getItem(
      notificationPreferencesStorageKey(scope),
    )
    if (!stored) {
      return { ...defaultNotificationDevicePreferences }
    }

    const parsed = JSON.parse(stored) as StoredNotificationDevicePreferences
    return {
      soundEnabled:
        typeof parsed.soundEnabled === "boolean" ? parsed.soundEnabled : true,
      permissionRequested: parsed.permissionRequested === true,
      permissionMenuClickedOn:
        typeof parsed.permissionMenuClickedOn === "string"
          ? parsed.permissionMenuClickedOn
          : "",
    }
  } catch (error) {
    console.warn("读取本机通知偏好失败", error)
    return { ...defaultNotificationDevicePreferences }
  }
}

/** 保存本机通知偏好并通知当前页面中的订阅方。 */
function writeNotificationDevicePreferences(
  scope: NotificationDeviceScope,
  preferences: NotificationDevicePreferences,
) {
  const storageKey = notificationPreferencesStorageKey(scope)
  try {
    window.localStorage.setItem(storageKey, JSON.stringify(preferences))
    window.dispatchEvent(
      new CustomEvent(notificationPreferencesChangedEvent, {
        detail: storageKey,
      }),
    )
  } catch (error) {
    console.warn("保存本机通知偏好失败", error)
  }
}

/** 保存当前企业用户在本设备上的通知声音开关。 */
export function setNotificationSoundEnabled(
  scope: NotificationDeviceScope,
  soundEnabled: boolean,
) {
  writeNotificationDevicePreferences(scope, {
    ...readNotificationDevicePreferences(scope),
    soundEnabled,
  })
}

/** 返回当前设备所在时区的自然日。 */
function currentDeviceCalendarDate() {
  const now = new Date()
  const year = now.getFullYear()
  const month = String(now.getMonth() + 1).padStart(2, "0")
  const day = String(now.getDate()).padStart(2, "0")
  return `${year}-${month}-${day}`
}

/** 订阅当前企业用户的本机通知偏好变化。 */
export function subscribeNotificationDevicePreferences(
  scope: NotificationDeviceScope,
  listener: () => void,
) {
  const storageKey = notificationPreferencesStorageKey(scope)

  /** 处理当前页面内主动派发的偏好变化。 */
  function handlePreferenceChange(event: Event) {
    if (
      event instanceof CustomEvent &&
      event.detail === storageKey
    ) {
      listener()
    }
  }

  /** 处理其他同源页面写入的偏好变化。 */
  function handleStorageChange(event: StorageEvent) {
    if (event.key === storageKey) {
      listener()
    }
  }

  window.addEventListener(
    notificationPreferencesChangedEvent,
    handlePreferenceChange,
  )
  window.addEventListener("storage", handleStorageChange)
  return () => {
    window.removeEventListener(
      notificationPreferencesChangedEvent,
      handlePreferenceChange,
    )
    window.removeEventListener("storage", handleStorageChange)
  }
}

/** 把各端权限结果转换为前端统一状态。 */
function normalizePermissionState(value: unknown): NotificationPermissionState {
  switch (value) {
    case "prompt":
    case "granted":
    case "denied":
    case "unsupported":
      return value
    case "default":
      return "prompt"
    case "system_managed":
    case "system-managed":
      return "system-managed"
    default:
      console.warn("未知的通知权限状态", value)
      return "unsupported"
  }
}

/** 返回当前端的通知权限状态。 */
export async function checkNotificationPermission(
  scope: NotificationDeviceScope,
): Promise<NotificationPermissionState> {
  const platform = resolveAppPlatform()
  if (platform === "mobile") {
    return "unsupported"
  }
  if (platform === "web") {
    if (!("Notification" in window) || !window.isSecureContext) {
      return "unsupported"
    }
    return normalizePermissionState(Notification.permission)
  }

  const state = normalizePermissionState(
    await checkDesktopNotificationPermission(),
  )
  if (
    state === "prompt" &&
    readNotificationDevicePreferences(scope).permissionRequested
  ) {
    return "denied"
  }
  return state
}

/** 在用户操作中申请当前端的通知权限。 */
export async function requestNotificationPermission(
  scope: NotificationDeviceScope,
): Promise<NotificationPermissionState> {
  const platform = resolveAppPlatform()
  let state: NotificationPermissionState
  try {
    if (platform === "mobile") {
      state = "unsupported"
    } else if (platform === "web") {
      if (!("Notification" in window) || !window.isSecureContext) {
        state = "unsupported"
      } else {
        state = normalizePermissionState(await Notification.requestPermission())
      }
    } else {
      state = normalizePermissionState(
        await requestDesktopNotificationPermission(),
      )
      if (state === "prompt") {
        state = "denied"
      }
    }
  } finally {
    writeNotificationDevicePreferences(scope, {
      ...readNotificationDevicePreferences(scope),
      permissionRequested: true,
    })
  }
  return state
}

/** 在支持的桌面系统中打开通知权限设置。 */
export async function openNotificationPermissionSettings() {
  if (!isDesktopMacOS()) {
    return false
  }
  await openExternalURL(macOSNotificationSettingsURL)
  return true
}

/** 点击消息菜单后每天最多直接申请一次通知权限。 */
export function requestNotificationPermissionFromMessageMenu(
  scope: NotificationDeviceScope,
) {
  const preferences = readNotificationDevicePreferences(scope)
  const today = currentDeviceCalendarDate()
  if (preferences.permissionMenuClickedOn === today) {
    return Promise.resolve(null)
  }

  writeNotificationDevicePreferences(scope, {
    ...preferences,
    permissionMenuClickedOn: today,
  })
  return requestNotificationPermission(scope)
}

/** 判断当前权限状态是否允许发送通知。 */
export function canSendNotification(state: NotificationPermissionState) {
  return state === "granted" || state === "system-managed"
}

/** 使用当前端执行已经通过提醒策略判断的新消息通知投递。 */
async function deliverMessageNotification(input: MessageNotificationInput) {
  const platform = resolveAppPlatform()
  if (platform === "mobile") {
    return
  }
  if (platform === "desktop") {
    await sendNativeMessageNotification(input)
    return
  }
  if (!("Notification" in window) || Notification.permission !== "granted") {
    throw new Error("浏览器尚未允许通知")
  }

  new Notification(input.title, {
    body: input.body,
    silent: !input.soundEnabled,
    tag: input.id,
  })
}

/** 按到达顺序以及账号、工作状态、权限和本机声音设置处理新消息。 */
export function notifyNewMessage(options: NewMessageNotificationOptions) {
  const policy = currentNotificationRuntimePolicy(options.scope)
  if (
    !policy ||
    !notificationRuntimePolicyAllowsAttention(policy) ||
    isApplicationVisible()
  ) {
    return Promise.resolve(false)
  }
  const generation = policy.generation

  const delivery = messageNotificationQueue.then(async () => {
    if (
      !notificationRuntimePolicyAllowsAttention(
        currentNotificationRuntimePolicy(options.scope, generation),
      ) ||
      isApplicationVisible()
    ) {
      return false
    }

    const permission = await checkNotificationPermission(options.scope)
    if (
      !canSendNotification(permission) ||
      !notificationRuntimePolicyAllowsAttention(
        currentNotificationRuntimePolicy(options.scope, generation),
      ) ||
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
    () => undefined,
  )
  return delivery
}

/** 按全局调用顺序把未读事实和注意力状态同步到桌面原生界面。 */
export function updateNotificationUnreadIndicator(
  state: UnreadIndicatorState,
) {
  if (resolveAppPlatform() !== "desktop") {
    return Promise.resolve()
  }

  const update = unreadIndicatorQueue.then(() =>
    updateDesktopUnreadIndicator(state),
  )
  unreadIndicatorQueue = update.catch(() => undefined)
  return update
}
