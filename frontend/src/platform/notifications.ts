/** Web 与桌面端通知权限、本机偏好、消息投递和未读提示适配。 */
import {
  NotificationPermissionStatus,
  checkNotificationPermission as checkDesktopNotificationPermission,
  requestNotificationPermission as requestDesktopNotificationPermission,
  sendNativeMessageNotification,
  updateUnreadIndicator as updateDesktopUnreadIndicator,
  type MessageNotificationInput,
  type UnreadIndicatorState,
} from "@/api"
import { isDesktopMacOS, resolveAppPlatform } from "@/platform/app-platform"
import { openExternalURL } from "@/platform/open-external-url"

const notificationPreferencesStoragePrefix = "cervi.notifications"
const macOSNotificationSettingsURL =
  "x-apple.systempreferences:com.apple.preference.notifications"
let unreadIndicatorQueue: Promise<void> = Promise.resolve()

export type NotificationPermissionState = Exclude<
  NotificationPermissionStatus,
  NotificationPermissionStatus.$zero
>

export type NotificationDeviceScope = {
  organizationId: string
  userId: string
}

export type NotificationDevicePreferences = {
  soundEnabled: boolean
  permissionMenuClickedOn: string
}

const defaultNotificationDevicePreferences: NotificationDevicePreferences = {
  soundEnabled: true,
  permissionMenuClickedOn: "",
}

/** 返回当前企业用户的通知偏好存储键。 */
function notificationPreferencesStorageKey(scope: NotificationDeviceScope) {
  return `${notificationPreferencesStoragePrefix}:${scope.organizationId}:${scope.userId}`
}

/** 读取当前企业用户的本机通知偏好。 */
export function readNotificationDevicePreferences(
  scope: NotificationDeviceScope,
): NotificationDevicePreferences {
  const storageKey = notificationPreferencesStorageKey(scope)
  try {
    const stored = window.localStorage.getItem(storageKey)
    if (!stored) {
      return { ...defaultNotificationDevicePreferences }
    }

    const parsed = JSON.parse(stored) as NotificationDevicePreferences
    if (
      typeof parsed.soundEnabled !== "boolean" ||
      typeof parsed.permissionMenuClickedOn !== "string"
    ) {
      throw new Error("invalid notification preferences")
    }
    return {
      soundEnabled: parsed.soundEnabled,
      permissionMenuClickedOn: parsed.permissionMenuClickedOn,
    }
  } catch (error) {
    console.warn("读取本机通知偏好失败", { storage_key: storageKey, error })
    return { ...defaultNotificationDevicePreferences }
  }
}

/** 保存当前企业用户的本机通知偏好。 */
function writeNotificationDevicePreferences(
  scope: NotificationDeviceScope,
  preferences: NotificationDevicePreferences,
) {
  const storageKey = notificationPreferencesStorageKey(scope)
  try {
    window.localStorage.setItem(storageKey, JSON.stringify(preferences))
  } catch (error) {
    console.warn("保存本机通知偏好失败", { storage_key: storageKey, error })
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

/** 返回当前设备的本地日期。 */
function currentDeviceCalendarDate() {
  const now = new Date()
  const year = now.getFullYear()
  const month = String(now.getMonth() + 1).padStart(2, "0")
  const day = String(now.getDate()).padStart(2, "0")
  return `${year}-${month}-${day}`
}

/** 返回当前端的通知权限状态。 */
export async function checkNotificationPermission(): Promise<
  NotificationPermissionState
> {
  const platform = resolveAppPlatform()
  if (platform === "mobile") {
    return NotificationPermissionStatus.NotificationPermissionStatusUnsupported
  }
  if (platform === "web") {
    if (!("Notification" in window) || !window.isSecureContext) {
      return NotificationPermissionStatus.NotificationPermissionStatusUnsupported
    }
    if (Notification.permission === "default") {
      return NotificationPermissionStatus.NotificationPermissionStatusPrompt
    }
    return Notification.permission === "granted"
      ? NotificationPermissionStatus.NotificationPermissionStatusGranted
      : NotificationPermissionStatus.NotificationPermissionStatusDenied
  }
  return (await checkDesktopNotificationPermission()) as NotificationPermissionState
}

/** 在用户操作中申请当前端的通知权限。 */
export async function requestNotificationPermission(): Promise<
  NotificationPermissionState
> {
  const platform = resolveAppPlatform()
  if (platform === "mobile") {
    return NotificationPermissionStatus.NotificationPermissionStatusUnsupported
  }
  if (platform === "web") {
    if (!("Notification" in window) || !window.isSecureContext) {
      return NotificationPermissionStatus.NotificationPermissionStatusUnsupported
    }
    const permission = await Notification.requestPermission()
    if (permission === "default") {
      return NotificationPermissionStatus.NotificationPermissionStatusPrompt
    }
    return permission === "granted"
      ? NotificationPermissionStatus.NotificationPermissionStatusGranted
      : NotificationPermissionStatus.NotificationPermissionStatusDenied
  }
  return (await requestDesktopNotificationPermission()) as NotificationPermissionState
}

/** 打开 macOS 通知设置。 */
export async function openNotificationPermissionSettings() {
  if (!isDesktopMacOS()) {
    return false
  }
  await openExternalURL(macOSNotificationSettingsURL)
  return true
}

/** 点击消息菜单后每天最多申请一次通知权限。 */
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
  return requestNotificationPermission()
}

/** 判断当前权限状态是否允许发送通知。 */
export function canSendNotification(state: NotificationPermissionState) {
  return (
    state === NotificationPermissionStatus.NotificationPermissionStatusGranted
  )
}

/** 投递一条新消息系统通知。 */
export async function deliverMessageNotification(
  input: MessageNotificationInput,
) {
  const platform = resolveAppPlatform()
  if (platform === "desktop") {
    await sendNativeMessageNotification(input)
    return
  }
  if (platform !== "web") {
    throw new Error("current platform does not support message notifications")
  }
  if (!("Notification" in window) || Notification.permission !== "granted") {
    throw new Error("browser notification permission is unavailable")
  }

  new Notification(input.title, {
    body: input.body,
    silent: !input.soundEnabled,
    tag: input.id,
  })
}

/** 按调用顺序更新桌面端未读提示。 */
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
