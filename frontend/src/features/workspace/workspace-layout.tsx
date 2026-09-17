/** Web 与桌面端工作台布局。 */
import { useEffect, useLayoutEffect, useRef, useState } from "react"
import { useTranslation } from "react-i18next"
import { Navigate, useLocation, useNavigate } from "react-router"
import { toast } from "sonner"

import { loadInbox, logout, WorkStatus } from "@/api"
import { AttachmentQueueProvider } from "@/features/inbox/attachment-queue-context"
import { OutgoingMessageProvider } from "@/features/inbox/outgoing-message-context"
import { LoadingIndicator } from "@/components/loading-indicator"
import { RealtimeSyncProvider } from "@/contexts/realtime-sync-context"
import { UserPreferencesProvider } from "@/contexts/user-preferences"
import {
  activateNotificationPolicy,
  deactivateNotificationPolicy,
} from "@/features/notifications/new-message-notifications"
import { useNewMessageNotifications } from "@/features/notifications/use-new-message-notifications"
import { useIdentityLoader } from "@/features/session/use-identity-loader"
import { useRealtimeConnection } from "@/features/session/use-realtime-connection"
import type { WorkspaceOutletContext } from "@/contexts/workspace-context"
import { WorkspaceNavigationGuard } from "@/features/workspace/workspace-navigation-guard"
import { WorkspaceNavigation } from "@/features/workspace/workspace-navigation"
import {
  defaultWorkspaceTab,
  resolveWorkspaceLocation,
  type ResolvedWorkspaceTab,
} from "@/features/workspace/workspace-page-routes"
import { WorkspaceSinglePage } from "@/features/workspace/workspace-single-page"
import { WorkspaceTabs } from "@/features/workspace/workspace-tabs"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"
import { resolveAppPlatform } from "@/platform/app-platform"
import { updateNotificationUnreadIndicator } from "@/platform/notifications"

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
  const [attentionPending, setAttentionPending] = useState(false)
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
  useRealtimeConnection(Boolean(userId))

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

  // 提醒总数按权威查询读取，会话变化由同步协调器失效该查询。
  const attention = useResource(
    resourceKeys.inboxAttention({ organizationId, userId }),
    async () => (await loadInbox({ limit: 1 })).attentionUnreadCount,
    { enabled: Boolean(organizationId && userId) },
  )
  const unreadCount = attention.data ?? 0

  // 实时确认的新消息按通知策略投递，投递成功即进入待处理提醒。
  const deliveredAt = useRef(0)
  useNewMessageNotifications(identity, () => {
    deliveredAt.current = Date.now()
    setAttentionPending(true)
  })

  /** 同步桌面端未读数和提醒状态。 */
  useEffect(() => {
    if (!userId || resolveAppPlatform() !== "desktop") {
      return
    }

    // 提醒总数的快照早于最近一次投递时仍是旧值，等待失效后的重读结果再判断是否清除待处理提醒。
    const settled = attention.dataUpdatedAt >= deliveredAt.current
    if ((!attentionEnabled || (unreadCount === 0 && settled)) && attentionPending) {
      setAttentionPending(false)
      return
    }
    void updateNotificationUnreadIndicator({
      count: unreadCount,
      attentionEnabled,
      attentionPending,
    }).catch((error) => {
      console.warn("同步桌面端未读状态失败", {
        count: unreadCount,
        attention_enabled: attentionEnabled,
        attention_pending: attentionPending,
        error,
      })
    })
  }, [userId, attentionEnabled, unreadCount, attentionPending, attention.dataUpdatedAt])

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
      setAttentionPending(false)
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
      !attentionPending ||
      document.visibilityState !== "visible" ||
      !document.hasFocus()
    ) {
      return
    }
    setAttentionPending(false)
  }, [location.pathname, attentionPending])

  /** 退出登录并回到登录页。 */
  async function handleLogout() {
    setLoggingOut(true)
    deactivateNotificationPolicy()
    setAttentionPending(false)
    // 先离开工作台，登出清空查询缓存时外壳已经卸载。
    navigate("/login", { replace: true })
    try {
      await logout()
    } catch (error) {
      console.warn("退出登录失败", error)
      toast.error(t("logoutError"))
    } finally {
      setLoggingOut(false)
    }
  }

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
  const workspaceContext = { identity } satisfies WorkspaceOutletContext
  const currentTab = workspaceLocation.tab ?? fallbackTabRef.current

  return (
    <UserPreferencesProvider user={identity.user}>
      <WorkspaceNavigationGuard
        tabsEnabled={identity.user.workspaceTabsEnabled}
      >
        <OutgoingMessageProvider key={identity.user.id}>
          <RealtimeSyncProvider>
            <AttachmentQueueProvider key={identity.user.id}>
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
            </AttachmentQueueProvider>
          </RealtimeSyncProvider>
        </OutgoingMessageProvider>
      </WorkspaceNavigationGuard>
    </UserPreferencesProvider>
  )
}
