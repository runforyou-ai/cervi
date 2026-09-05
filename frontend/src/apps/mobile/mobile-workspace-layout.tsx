/** 移动端身份入口、一级导航和详情布局。 */
import { createContext, useContext } from "react"
import { ContactRoundIcon, InboxIcon, UserRoundIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { Navigate, NavLink, Outlet } from "react-router"

import type { Identity } from "@/api"
import {
  MobileNavigationProvider,
  useMobileNavigation,
} from "@/apps/mobile/mobile-navigation"
import { LoadingIndicator } from "@/components/loading-indicator"
import { UserPreferencesProvider } from "@/contexts/user-preferences"
import { useIdentityLoader } from "@/features/session/use-identity-loader"
import { cn } from "@/lib/utils"

const MobileWorkspaceContext = createContext<Identity | null>(null)

/** 加载当前身份并为所有移动端页面提供公共上下文。 */
export function MobileWorkspaceLayout() {
  const { t } = useTranslation("mobile")
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
        <LoadingIndicator>{t("loading")}</LoadingIndicator>
      </main>
    )
  }
  return (
    <MobileWorkspaceContext value={identity}>
      <UserPreferencesProvider user={identity.user}>
        <MobileNavigationProvider>
          <div className="flex h-dvh min-h-0 flex-col overflow-hidden bg-background pt-[env(safe-area-inset-top)]">
            <Outlet />
          </div>
        </MobileNavigationProvider>
      </UserPreferencesProvider>
    </MobileWorkspaceContext>
  )
}

/** 为一级页面显示固定底部导航。 */
export function MobileTabLayout() {
  const { t } = useTranslation("mobile")
  const { inboxURL } = useMobileNavigation()
  const tabs = [
    { path: inboxURL, label: t("tabs.inbox"), icon: InboxIcon },
    { path: "/contacts", label: t("tabs.contacts"), icon: ContactRoundIcon },
    { path: "/me", label: t("tabs.me"), icon: UserRoundIcon },
  ]
  return (
    <>
      <main className="min-h-0 flex-1 overflow-hidden">
        <Outlet />
      </main>
      <nav
        aria-label={t("tabs.label")}
        className="shrink-0 border-t bg-background pb-[env(safe-area-inset-bottom)]"
      >
        <div className="grid grid-cols-3">
          {tabs.map(({ path, label, icon: Icon }) => (
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
              <Icon className="size-5" />
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
    <main className="min-h-0 flex-1 overflow-hidden pb-[env(safe-area-inset-bottom)]">
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
