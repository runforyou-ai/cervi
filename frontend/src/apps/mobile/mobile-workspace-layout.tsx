/** 移动端身份入口、一级导航和详情布局。 */
import { createContext, useCallback, useContext, useState } from "react"
import { ContactRoundIcon, InboxIcon, UserRoundIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { Navigate, NavLink, Outlet, useLocation } from "react-router"

import { loadInbox, type Identity } from "@/api"
import {
  MobileNavigationProvider,
  useMobileNavigation,
} from "@/apps/mobile/mobile-navigation"
import { LoadingIndicator } from "@/components/loading-indicator"
import { UserPreferencesProvider } from "@/contexts/user-preferences"
import { AttachmentQueueProvider } from "@/features/inbox/attachment-queue-context"
import { OutgoingMessageProvider } from "@/features/inbox/outgoing-message-context"
import {
  memberChatPollingInterval,
  useMemberChatPollingActive,
} from "@/features/inbox/use-member-chat-polling"
import { useIdentityLoader } from "@/features/session/use-identity-loader"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"
import { cn } from "@/lib/utils"

const MobileWorkspaceContext = createContext<Identity | null>(null)

/** 一级页面向底部导航提供的回调。 */
export type MobileTabContext = {
  reportInboxAttention: (count: number) => void
}

/** 加载当前身份并为所有移动端页面提供公共上下文。 */
export function MobileWorkspaceLayout() {
  const { t } = useTranslation(["mobile", "common"])
  const { status, identity, redirectPath } = useIdentityLoader()
  if (status === "anonymous") return <Navigate to="/login" replace />
  if (status === "redirect" && redirectPath)
    return <Navigate to={redirectPath} replace />
  if (status === "failed") {
    return (
      <main className="flex min-h-dvh items-center justify-center px-6 text-center text-sm text-muted-foreground">
        {t("identityLoadError")}
      </main>
    )
  }
  if (!identity) {
    return (
      <main className="flex min-h-dvh items-center justify-center">
        <LoadingIndicator>{t("common:status.loading")}</LoadingIndicator>
      </main>
    )
  }
  return (
    <MobileWorkspaceContext value={identity}>
      <UserPreferencesProvider user={identity.user}>
        <MobileNavigationProvider>
          <OutgoingMessageProvider key={identity.user.id}>
            <AttachmentQueueProvider>
              <div className="flex h-dvh min-h-0 flex-col overflow-hidden bg-sidebar pt-[env(safe-area-inset-top)]">
                <Outlet />
              </div>
            </AttachmentQueueProvider>
          </OutgoingMessageProvider>
        </MobileNavigationProvider>
      </UserPreferencesProvider>
    </MobileWorkspaceContext>
  )
}

/** 为一级页面显示固定底部导航和消息页签的内部提醒角标。 */
export function MobileTabLayout() {
  const { t } = useTranslation(["mobile", "inbox"])
  const { inboxURL } = useMobileNavigation()
  const { identity } = useMobileWorkspace()
  const { pathname } = useLocation()
  const pollingActive = useMemberChatPollingActive({
    requireWindowFocus: false,
  })
  const [listAttention, setListAttention] = useState<{
    count: number
    at: number
  } | null>(null)
  const reportInboxAttention = useCallback((count: number) => {
    setListAttention({ count, at: Date.now() })
  }, [])
  // 消息页由收件箱列表上报提醒总数，其他一级页面读取一条会话取得总数。
  const attention = useResource(
    resourceKeys.inboxAttention({
      organizationId: identity.organization.id,
      userId: identity.user.id,
    }),
    async () => (await loadInbox({ limit: 1 })).attentionUnreadCount,
    {
      enabled: pathname !== "/inbox",
      refetchInterval: pollingActive ? memberChatPollingInterval : false,
    },
  )
  // 采用列表上报与页签读取中较新的提醒总数。
  const attentionUnreadCount =
    listAttention && listAttention.at > attention.dataUpdatedAt
      ? listAttention.count
      : (attention.data ?? 0)
  const tabs = [
    {
      path: inboxURL,
      label: t("tabs.inbox"),
      icon: InboxIcon,
      badge: attentionUnreadCount,
    },
    { path: "/contacts", label: t("tabs.contacts"), icon: ContactRoundIcon },
    { path: "/me", label: t("tabs.me"), icon: UserRoundIcon },
  ]
  return (
    <>
      <main className="min-h-0 flex-1 overflow-hidden bg-background">
        <Outlet
          context={{ reportInboxAttention } satisfies MobileTabContext}
        />
      </main>
      <nav
        aria-label={t("tabs.label")}
        className="shrink-0 border-t bg-sidebar pb-[env(safe-area-inset-bottom)]"
      >
        <div className="grid grid-cols-3">
          {tabs.map(({ path, label, icon: Icon, badge }) => (
            <NavLink
              key={label}
              to={path}
              replace
              className={({ isActive }) =>
                cn(
                  "flex min-h-14 flex-col items-center justify-center gap-0.5 px-3 text-[11px] font-medium text-muted-foreground outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring",
                  isActive && "text-primary",
                )
              }
            >
              <span className="relative">
                <Icon className="size-5" />
                {badge ? (
                  <span className="absolute -top-1.5 left-3 flex h-4 min-w-4 items-center justify-center rounded-full bg-destructive px-1 text-[10px] leading-4 font-semibold text-destructive-foreground ring-2 ring-sidebar">
                    <span aria-hidden="true">{badge > 99 ? "99+" : badge}</span>
                    <span className="sr-only">
                      {t("inbox:internalAttentionUnread", { count: badge })}
                    </span>
                  </span>
                ) : null}
              </span>
              <span>{label}</span>
            </NavLink>
          ))}
        </div>
      </nav>
    </>
  )
}

/** 为详情页面保留底部安全区并隐藏一级导航。 */
export function MobileDetailLayout() {
  return (
    <main className="min-h-0 flex-1 overflow-hidden bg-background pb-[env(safe-area-inset-bottom)]">
      <Outlet />
    </main>
  )
}

/** 返回移动端工作区中的当前身份。 */
export function useMobileWorkspace() {
  const identity = useContext(MobileWorkspaceContext)
  if (!identity) throw new Error("移动端页面必须位于登录工作区内")
  return { identity }
}
