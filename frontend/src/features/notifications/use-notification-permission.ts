/** 在 React 页面中同步当前设备的通知权限状态。 */
import { useEffect, useRef, useState } from "react"

import {
  checkNotificationPermission,
  requestNotificationPermission,
  subscribeNotificationDevicePreferences,
  type NotificationDeviceScope,
  type NotificationPermissionState,
} from "@/platform/notifications"

export type NotificationPermissionViewState =
  | NotificationPermissionState
  | "checking"

/** 读取、刷新并申请当前企业用户在本设备上的通知权限。 */
export function useNotificationPermission(scope: NotificationDeviceScope) {
  const [status, setStatus] =
    useState<NotificationPermissionViewState>("checking")
  const [requesting, setRequesting] = useState(false)
  const activeScopeRef = useRef("")
  const scopeKey = `${scope.organizationId}:${scope.userId}`

  useEffect(() => {
    let active = true
    const currentScope = {
      organizationId: scope.organizationId,
      userId: scope.userId,
    }
    activeScopeRef.current = scopeKey

    /** 重新读取当前设备的通知权限。 */
    async function refreshPermission() {
      try {
        const nextStatus = await checkNotificationPermission(currentScope)
        if (active) {
          setStatus(nextStatus)
        }
      } catch (error) {
        if (active) {
          console.warn("读取通知权限失败", error)
          setStatus("unsupported")
        }
      }
    }

    setStatus("checking")
    setRequesting(false)
    void refreshPermission()
    const unsubscribe = subscribeNotificationDevicePreferences(
      currentScope,
      () => void refreshPermission(),
    )
    window.addEventListener("focus", refreshPermission)
    return () => {
      active = false
      if (activeScopeRef.current === scopeKey) {
        activeScopeRef.current = ""
      }
      unsubscribe()
      window.removeEventListener("focus", refreshPermission)
    }
  }, [scope.organizationId, scope.userId, scopeKey])

  /** 从当前用户操作中申请通知权限并同步最新状态。 */
  async function requestPermission() {
    const requestedScope = {
      organizationId: scope.organizationId,
      userId: scope.userId,
    }
    const requestedScopeKey = scopeKey
    setRequesting(true)
    try {
      const nextStatus = await requestNotificationPermission(requestedScope)
      if (activeScopeRef.current !== requestedScopeKey) {
        return null
      }
      setStatus(nextStatus)
      return nextStatus
    } catch (error) {
      if (activeScopeRef.current !== requestedScopeKey) {
        return null
      }
      throw error
    } finally {
      if (activeScopeRef.current === requestedScopeKey) {
        setRequesting(false)
      }
    }
  }

  return { status, requesting, requestPermission }
}
