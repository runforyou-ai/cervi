/** 移动端登录后工作区和底部一级导航。 */
import { InboxIcon, LoaderCircleIcon, UserRoundIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import {
  Navigate,
  NavLink,
  Outlet,
  useOutletContext,
} from "react-router"

import type { Identity } from "@/api"
import { UserPreferencesProvider } from "@/contexts/user-preferences"
import { useIdentityLoader } from "@/features/session/use-identity-loader"
import { cn } from "@/lib/utils"

type MobileWorkspaceContext = {
  identity: Identity
}

const mobileTabs = [
  { path: "/inbox", labelKey: "tabs.inbox", icon: InboxIcon },
  { path: "/me", labelKey: "tabs.me", icon: UserRoundIcon },
] as const

/** 渲染一个移动端一级导航入口。 */
function MobileTab({
  path,
  label,
  icon: Icon,
}: {
  path: string
  label: string
  icon: typeof InboxIcon
}) {
  return (
    <NavLink
      to={path}
      replace
      className={({ isActive }) =>
        cn(
          "flex min-h-14 flex-col items-center justify-center gap-0.5 px-3 text-[11px] font-medium text-muted-foreground outline-none transition-colors focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring",
          isActive && "text-primary",
        )
      }
    >
      <Icon className="size-5" />
      <span>{label}</span>
    </NavLink>
  )
}

/** 加载一次当前身份并渲染移动端工作区。 */
export function MobileWorkspaceLayout() {
  const { t } = useTranslation("mobile")
  const { status, identity, redirectPath } = useIdentityLoader()

  if (status === "anonymous") return <Navigate to="/login" replace />
  if (status === "redirect" && redirectPath) {
    return <Navigate to={redirectPath} replace />
  }
  if (status === "failed") {
    return (
      <main className="flex min-h-dvh items-center justify-center px-6 text-center">
        <p className="text-sm text-muted-foreground">
          {t("identityLoadError")}
        </p>
      </main>
    )
  }
  if (!identity) {
    return (
      <main className="flex min-h-dvh items-center justify-center gap-2 text-sm text-muted-foreground">
        <LoaderCircleIcon className="size-4 animate-spin" />
        {t("loading")}
      </main>
    )
  }

  return (
    <UserPreferencesProvider user={identity.user}>
      <div className="flex h-dvh min-h-0 flex-col overflow-hidden bg-background pt-[env(safe-area-inset-top)]">
        <div className="min-h-0 flex-1 overflow-hidden">
          <Outlet context={{ identity } satisfies MobileWorkspaceContext} />
        </div>
        <nav
          aria-label={t("tabs.label")}
          className="shrink-0 border-t bg-background pb-[env(safe-area-inset-bottom)]"
        >
          <div className="grid grid-cols-2">
            {mobileTabs.map((tab) => (
              <MobileTab
                key={tab.path}
                path={tab.path}
                label={t(tab.labelKey)}
                icon={tab.icon}
              />
            ))}
          </div>
        </nav>
      </div>
    </UserPreferencesProvider>
  )
}

/** 返回移动端工作区中的当前身份。 */
export function useMobileWorkspace() {
  return useOutletContext<MobileWorkspaceContext>()
}
