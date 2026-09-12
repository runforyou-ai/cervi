/** Web 与桌面端工作台布局。 */
import { useCallback, useEffect, useLayoutEffect, useRef, useState } from "react"
import { useTranslation } from "react-i18next"
import { Navigate, useLocation, useNavigate } from "react-router"
import { toast } from "sonner"

import { logout, WorkStatus } from "@/api"
import { AttachmentQueueProvider } from "@/features/inbox/attachment-queue-context"
import { OutgoingMessageProvider } from "@/features/inbox/outgoing-message-context"
import { LoadingIndicator } from "@/components/loading-indicator"
import { UserPreferencesProvider } from "@/contexts/user-preferences"
import {
  activateNotificationPolicy,
  deactivateNotificationPolicy,
  notifyNewMessage as deliverNewMessageNotification,
} from "@/features/notifications/new-message-notifications"
import { useIdentityLoader } from "@/features/session/use-identity-loader"
import type {
  WorkspaceNewMessageNotification,
  WorkspaceOutletContext,
} from "@/contexts/workspace-context"
import { WorkspaceNavigationGuard } from "@/features/workspace/workspace-navigation-guard"
import { WorkspaceNavigation } from "@/features/workspace/workspace-navigation"
import {
  defaultWorkspaceTab,
  resolveWorkspaceLocation,
  type ResolvedWorkspaceTab,
} from "@/features/workspace/workspace-page-routes"
import { WorkspaceSinglePage } from "@/features/workspace/workspace-single-page"
import { WorkspaceTabs } from "@/features/workspace/workspace-tabs"
import { resolveAppPlatform } from "@/platform/app-platform"
import { updateNotificationUnreadIndicator } from "@/platform/notifications"

type WorkspaceUnreadState = {
  count: number
  attentionPending: boolean
}

/** 页面导航后清除非编辑区域的文字选区。 */
function useClearSelectionOnNavigation() {
  const location = useLocation()

  useLayoutEffect(() => {
    const activeElement = document.activeElement
    if (
      activeElement instanceof HTMLInputElement ||
      activeElement instanceof HTMLTextAreaElement ||
      (activeElement instanceof HTMLElement && activeElement.isContentEditable)
    ) {
      return
    }
    window.getSelection()?.removeAllRanges()
  }, [location.key])
}

/** 读取登录身份并渲染工作台导航和子页面。 */
export function WorkspaceLayout() {
  useClearSelectionOnNavigation()
  const location = useLocation()
  const { t } = useTranslation(["workspace", "common"])
  const navigate = useNavigate()
  const [loggingOut, setLoggingOut] = useState(false)
  const [unreadState, setUnreadState] = useState<WorkspaceUnreadState>({
    count: 0,
    attentionPending: false,
  })
  const unreadRevisionRef = useRef(0)
  const { status, identity, redirectPath } = useIdentityLoader()
  const workspaceLocation = resolveWorkspaceLocation(location)
  const fallbackTabRef = useRef<ResolvedWorkspaceTab>(defaultWorkspaceTab)
  const currentHref = `${location.pathname}${location.search}${location.hash}`
  if (
    workspaceLocation.tab &&
    workspaceLocation.canonicalHref === currentHref
  ) {
    fallbackTabRef.current = workspaceLocation.tab
  }

  const organizationId = identity?.user.organizationId
  const userId = identity?.user.id
  const messageNotificationsEnabled = identity?.user.messageNotificationsEnabled
  const workStatus = identity?.user.workStatus

  /** 修正规范工作台地址。 */
  useLayoutEffect(() => {
    if (
      !userId ||
      (workspaceLocation.tab && workspaceLocation.canonicalHref === currentHref)
    ) {
      return
    }
    navigate(workspaceLocation.canonicalHref, { replace: true })
  }, [
    currentHref,
    userId,
    navigate,
    workspaceLocation.canonicalHref,
    workspaceLocation.tab,
  ])

  /** 同步当前用户的新消息通知策略。 */
  useLayoutEffect(() => {
    if (
      !organizationId ||
      !userId ||
      messageNotificationsEnabled === undefined ||
      workStatus === undefined
    ) {
      return
    }
    return activateNotificationPolicy(
      { organizationId, userId },
      messageNotificationsEnabled,
      workStatus,
    )
  }, [organizationId, userId, messageNotificationsEnabled, workStatus])

  const attentionEnabled =
    Boolean(messageNotificationsEnabled) &&
    workStatus === WorkStatus.WorkStatusWorking

  /** 同步桌面端未读数和提醒状态。 */
  useEffect(() => {
    if (!userId || resolveAppPlatform() !== "desktop") {
      return
    }

    if (!attentionEnabled && unreadState.attentionPending) {
      setUnreadState((current) => ({
        ...current,
        attentionPending: false,
      }))
      return
    }
    void updateNotificationUnreadIndicator({
      count: unreadState.count,
      attentionEnabled,
      attentionPending: unreadState.attentionPending,
    }).catch((error) => {
      console.warn("同步桌面端未读状态失败", {
        count: unreadState.count,
        attention_enabled: attentionEnabled,
        attention_pending: unreadState.attentionPending,
        error,
      })
    })
  }, [userId, attentionEnabled, unreadState.count, unreadState.attentionPending])

  /** 用户重新查看应用时停止托盘闪烁。 */
  useEffect(() => {
    if (resolveAppPlatform() !== "desktop") {
      return
    }

    /** 清除待处理的托盘提醒。 */
    function clearAttention() {
      if (
        document.visibilityState !== "visible" ||
        !document.hasFocus()
      ) {
        return
      }
      setUnreadState((current) =>
        current.attentionPending
          ? { ...current, attentionPending: false }
          : current,
      )
    }

    window.addEventListener("focus", clearAttention)
    document.addEventListener("visibilitychange", clearAttention)
    return () => {
      window.removeEventListener("focus", clearAttention)
      document.removeEventListener("visibilitychange", clearAttention)
      void updateNotificationUnreadIndicator({
        count: 0,
        attentionEnabled: false,
        attentionPending: false,
      }).catch((error) => {
        console.warn("清除桌面端未读状态失败", error)
      })
    }
  }, [])

  /** 用户查看消息页时停止托盘闪烁。 */
  useEffect(() => {
    if (
      location.pathname !== "/inbox" ||
      !unreadState.attentionPending ||
      document.visibilityState !== "visible" ||
      !document.hasFocus()
    ) {
      return
    }
    setUnreadState((current) => ({
      ...current,
      attentionPending: false,
    }))
  }, [location.pathname, unreadState.attentionPending])

  /** 退出登录并回到登录页。 */
  async function handleLogout() {
    setLoggingOut(true)
    deactivateNotificationPolicy()
    unreadRevisionRef.current += 1
    setUnreadState({ count: 0, attentionPending: false })
    // 先离开工作台，登出清空查询缓存时外壳已经卸载。
    navigate("/login", { replace: true })
    try {
      await logout()
      console.info("用户退出登录")
    } catch (error) {
      console.warn("退出登录失败", error)
      toast.error(t("logoutError"))
    } finally {
      setLoggingOut(false)
    }
  }

  /** 返回当前未读状态修订号。 */
  const beginUnreadSnapshot = useCallback(function beginUnreadSnapshot() {
    return unreadRevisionRef.current
  }, [])

  /** 应用未被实时消息覆盖的未读快照。 */
  const applyUnreadSnapshot = useCallback(function applyUnreadSnapshot(
    count: number,
    revision: number,
  ) {
    if (revision !== unreadRevisionRef.current) {
      return
    }
    const normalizedCount = Math.max(0, count)
    setUnreadState((current) => ({
      count: normalizedCount,
      attentionPending:
        normalizedCount > 0 && current.attentionPending,
    }))
  }, [])

  /** 更新实时未读数并处理新消息提醒。 */
  const notifyNewMessage = useCallback(async function notifyNewMessage(
    notification: WorkspaceNewMessageNotification,
  ) {
    if (!organizationId || !userId) {
      return false
    }

    unreadRevisionRef.current += 1
    const unreadCount = Math.max(0, notification.unreadCount)
    const alreadyVisible =
      document.visibilityState === "visible" && document.hasFocus()
    setUnreadState({
      count: unreadCount,
      attentionPending:
        unreadCount > 0 && attentionEnabled && !alreadyVisible,
    })

    if (unreadCount === 0) {
      return false
    }

    return deliverNewMessageNotification({
      id: notification.id,
      title: notification.title,
      body: notification.body,
      scope: { organizationId, userId },
    })
  }, [organizationId, userId, attentionEnabled])

  if (status === "anonymous") return <Navigate to="/login" replace />
  if (status === "redirect" && redirectPath) {
    return <Navigate to={redirectPath} replace />
  }
  if (status === "failed") {
    return (
      <main className="flex min-h-svh items-center justify-center text-sm text-muted-foreground">
        {t("identityLoadError")}
      </main>
    )
  }
  if (!identity) {
    return (
      <main className="flex min-h-svh items-center justify-center">
        <LoadingIndicator>{t("common:status.loading")}</LoadingIndicator>
      </main>
    )
  }
  const workspaceContext = {
    identity,
    beginUnreadSnapshot,
    applyUnreadSnapshot,
    notifyNewMessage,
  } satisfies WorkspaceOutletContext
  const currentTab = workspaceLocation.tab ?? fallbackTabRef.current

  return (
    <UserPreferencesProvider user={identity.user}>
      <WorkspaceNavigationGuard
        tabsEnabled={identity.user.workspaceTabsEnabled}
      >
        <AttachmentQueueProvider key={identity.user.id}>
          <OutgoingMessageProvider key={identity.user.id}>
            <div className="cervi-workspace-shell relative flex h-svh min-h-0 w-full overflow-hidden">
              <WorkspaceNavigation
                identity={identity}
                onLogout={handleLogout}
                loggingOut={loggingOut}
              />
              <div
                aria-hidden="true"
                className="cervi-workspace-top-drag-region"
              />
              <div className="cervi-workspace-content-frame flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden rounded-xl bg-background shadow-sm">
                {identity.user.workspaceTabsEnabled ? (
                  <WorkspaceTabs
                    currentTab={currentTab}
                    context={workspaceContext}
                  />
                ) : (
                  <WorkspaceSinglePage
                    href={currentTab.href}
                    context={workspaceContext}
                  />
                )}
              </div>
            </div>
          </OutgoingMessageProvider>
        </AttachmentQueueProvider>
      </WorkspaceNavigationGuard>
    </UserPreferencesProvider>
  )
}
