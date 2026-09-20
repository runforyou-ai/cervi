/** Web 与桌面端工作台布局。 */
import { useEffect, useLayoutEffect, useRef, useState, type CSSProperties } from "react"
import { useTranslation } from "react-i18next"
import { useLocation, useNavigate } from "react-router"
import { toast } from "sonner"

import { loadInbox, logout, WorkStatus, type Identity } from "@/api"
import {
  WorkspaceProvider,
  type WorkspaceOutletContext,
} from "@/contexts/workspace-context"
import {
  activateNotificationPolicy,
  deactivateNotificationPolicy,
} from "@/features/notifications/new-message-notifications"
import { useNewMessageNotifications } from "@/features/notifications/use-new-message-notifications"
import { SessionShell } from "@/features/session/session-shell"
import {
  useWorkspaceHistory,
  WorkspaceHistoryNav,
} from "@/features/workspace/workspace-history-nav"
import { WorkspaceNavigationGuard } from "@/features/workspace/workspace-navigation-guard"
import { WorkspaceNavigation } from "@/features/workspace/workspace-navigation"
import {
  defaultWorkspaceHref,
  resolveWorkspaceLocation,
  WorkspacePageRoutes,
} from "@/features/workspace/workspace-page-routes"
import {
  useWorkspaceRail,
  WorkspaceRailResizer,
  WorkspaceRailToggle,
} from "@/features/workspace/workspace-rail"
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

/** 在登录外壳内渲染工作台。 */
export function WorkspaceLayout() {
  return (
    <SessionShell>
      {(identity) => <WorkspaceShell identity={identity} />}
    </SessionShell>
  )
}

/** 按登录身份渲染工作台导航、子页面和桌面端提醒状态。 */
function WorkspaceShell({ identity }: { identity: Identity }) {
  useClearSelectionOnNavigation()
  const location = useLocation()
  const { t } = useTranslation("workspace")
  const navigate = useNavigate()
  const [loggingOut, setLoggingOut] = useState(false)
  const rail = useWorkspaceRail()
  const history = useWorkspaceHistory()
  const [attentionPending, setAttentionPending] = useState(false)
  const workspaceLocation = resolveWorkspaceLocation(location)
  const fallbackHrefRef = useRef(defaultWorkspaceHref)
  const currentHref = `${location.pathname}${location.search}${location.hash}`
  if (workspaceLocation.matched && workspaceLocation.canonicalHref === currentHref) {
    fallbackHrefRef.current = currentHref
  }

  const organizationId = identity.user.organizationId
  const userId = identity.user.id
  const messageNotificationsEnabled = identity.user.messageNotificationsEnabled
  const workStatus = identity.user.workStatus

  /** 修正规范工作台地址。 */
  useLayoutEffect(() => {
    if (workspaceLocation.matched && workspaceLocation.canonicalHref === currentHref) {
      return
    }
    navigate(workspaceLocation.canonicalHref, { replace: true })
  }, [
    currentHref,
    navigate,
    workspaceLocation.canonicalHref,
    workspaceLocation.matched,
  ])

  /** 同步当前用户的新消息通知策略。 */
  useLayoutEffect(
    () =>
      activateNotificationPolicy(
        { organizationId, userId },
        messageNotificationsEnabled,
        workStatus,
      ),
    [organizationId, userId, messageNotificationsEnabled, workStatus],
  )

  const attentionEnabled =
    messageNotificationsEnabled && workStatus === WorkStatus.WorkStatusWorking

  // 提醒总数按权威查询读取，会话变化由同步协调器失效该查询。
  const attention = useResource(
    resourceKeys.inboxAttention({ organizationId, userId }),
    async () => {
      // 应用角标合计内部会话提醒与客户会话中提醒本人的未读。
      const inbox = await loadInbox({ limit: 1 })
      return inbox.attentionUnreadCount + inbox.customerMentionedUnreadCount
    },
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
    if (resolveAppPlatform() !== "desktop") {
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
  }, [attentionEnabled, unreadCount, attentionPending, attention.dataUpdatedAt])

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

  const workspaceContext = { identity } satisfies WorkspaceOutletContext
  const pageHref = workspaceLocation.matched
    ? workspaceLocation.canonicalHref
    : fallbackHrefRef.current

  return (
    <WorkspaceNavigationGuard>
      <div
        className="cervi-workspace-shell relative flex h-svh min-h-0 w-full overflow-hidden"
        data-rail-collapsed={rail.collapsed ? "true" : undefined}
        style={
          {
            "--cervi-workspace-rail-width": rail.collapsed
              ? "0px"
              : `${rail.width}px`,
          } as CSSProperties
        }
      >
        {rail.collapsed ? null : (
          <>
            <WorkspaceNavigation
              identity={identity}
              onLogout={handleLogout}
              loggingOut={loggingOut}
            />
            <WorkspaceRailResizer onWidthChange={rail.changeWidth} />
          </>
        )}
        <div className="cervi-workspace-titlebar-actions absolute top-0 left-0 z-40 flex items-center">
          <WorkspaceRailToggle
            collapsed={rail.collapsed}
            onToggle={rail.toggleCollapsed}
          />
          <WorkspaceHistoryNav
            canGoBack={history.canGoBack}
            canGoForward={history.canGoForward}
            onBack={history.goBack}
            onForward={history.goForward}
          />
        </div>
        <div
          aria-hidden="true"
          className="cervi-workspace-top-drag-region"
        />
        <div className="cervi-workspace-content-frame relative flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden rounded-xl bg-background shadow-sm">
          <WorkspaceProvider value={workspaceContext}>
            <WorkspacePageRoutes location={pageHref} />
          </WorkspaceProvider>
        </div>
      </div>
    </WorkspaceNavigationGuard>
  )
}
