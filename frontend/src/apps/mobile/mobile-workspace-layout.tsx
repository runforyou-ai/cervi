/** 移动端身份入口、一级导航和详情布局。 */
import { createContext, useContext } from "react"
import { ContactRoundIcon, InboxIcon, MessageCircleIcon, UserRoundIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { NavLink, Outlet } from "react-router"

import { type Identity } from "@/api"
import { useMobileMessageNotifications } from "@/apps/mobile/mobile-message-notifications"
import {
  MobileNavigationProvider,
  useMobileNavigation,
} from "@/apps/mobile/mobile-navigation"
import { useRealtimeSyncActive } from "@/contexts/realtime-sync-context"
import {
  memberChatPollingInterval,
  useMemberChatPollingActive,
} from "@/features/inbox/use-member-chat-polling"
import { UnsavedChangesGuard } from "@/components/unsaved-changes-guard"
import { loadInboxAttention } from "@/features/inbox/inbox-attention"
import { SessionShell } from "@/features/session/session-shell"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"
import { cn } from "@/lib/utils"

const MobileWorkspaceContext = createContext<Identity | null>(null)

/** 在登录工作区内挂载新消息系统通知与应用角标。 */
function MobileMessageNotifications({ identity }: { identity: Identity }) {
  useMobileMessageNotifications(identity)
  return null
}

/** 在登录外壳内为所有移动端页面提供身份、导航上下文和未保存内容确认，回到前台时重建实时事件流。 */
export function MobileWorkspaceLayout() {
  return (
    <SessionShell restartOnResume>
      {(identity) => (
        <MobileWorkspaceContext value={identity}>
          <MobileMessageNotifications identity={identity} />
          <MobileNavigationProvider>
            <UnsavedChangesGuard>
              <div className="flex h-dvh min-h-0 flex-col overflow-hidden bg-sidebar pt-[env(safe-area-inset-top)]">
                <Outlet />
              </div>
            </UnsavedChangesGuard>
          </MobileNavigationProvider>
        </MobileWorkspaceContext>
      )}
    </SessionShell>
  )
}

/** 为一级页面显示固定底部导航，收件箱显示待处理会话中的未读消息数，消息显示聊天提醒未读数。 */
export function MobileTabLayout() {
  const { t } = useTranslation(["mobile", "inbox"])
  const { chatsURL, inboxURL } = useMobileNavigation()
  const { identity } = useMobileWorkspace()
  const pollingActive = useMemberChatPollingActive({
    requireWindowFocus: false,
  })
  const realtime = useRealtimeSyncActive()
  // 提醒总数按权威查询读取，会话变化由同步协调器失效该查询。
  const attention = useResource(
    resourceKeys.inboxAttention({
      organizationId: identity.organization.id,
      userId: identity.user.id,
    }),
    loadInboxAttention,
    {
      refetchInterval:
        pollingActive && !realtime ? memberChatPollingInterval : false,
    },
  )
  const tabs = [
    {
      path: inboxURL,
      label: t("tabs.inbox"),
      icon: InboxIcon,
      badge: attention.data?.pendingUnread ?? 0,
      badgeLabel: t("inbox:pendingUnreadCount", { count: attention.data?.pendingUnread ?? 0 }),
    },
    {
      path: chatsURL,
      label: t("tabs.chats"),
      icon: MessageCircleIcon,
      badge: attention.data?.unread ?? 0,
      badgeLabel: t("inbox:chatAttentionUnread", { count: attention.data?.unread ?? 0 }),
    },
    { path: "/contacts", label: t("tabs.contacts"), icon: ContactRoundIcon },
    { path: "/me", label: t("tabs.me"), icon: UserRoundIcon },
  ]
  return (
    <>
      <main className="min-h-0 flex-1 overflow-hidden bg-background">
        <Outlet />
      </main>
      <nav
        aria-label={t("tabs.label")}
        className="shrink-0 border-t bg-sidebar pb-[env(safe-area-inset-bottom)]"
      >
        <div className="grid grid-cols-4">
          {tabs.map(({ path, label, icon: Icon, badge, badgeLabel }) => (
            <NavLink
              key={label}
              to={path}
              replace
              className={({ isActive }) =>
                cn(
                  "flex min-h-14 flex-col items-center justify-center gap-0.5 px-3 text-xs font-medium text-muted-foreground outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring",
                  isActive && "text-primary",
                )
              }
            >
              <span className="relative">
                <Icon className="size-5" />
                {badge ? (
                  <span className="absolute -top-1.5 left-3 flex h-4 min-w-4 items-center justify-center rounded-full bg-destructive px-1 text-[10px] leading-4 font-semibold text-destructive-foreground ring-2 ring-sidebar">
                    <span aria-hidden="true">{badge > 99 ? "99+" : badge}</span>
                    <span className="sr-only">{badgeLabel}</span>
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
