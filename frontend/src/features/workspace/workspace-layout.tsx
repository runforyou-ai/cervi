/** Web 与桌面端工作台布局。 */
import { useEffect, useLayoutEffect, useState } from "react"
import { LoaderCircleIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { Outlet, useLocation, useNavigate } from "react-router"
import { toast } from "sonner"

import {
  logout,
  sessionPath,
  SessionState,
  type Identity,
  type User,
} from "@/api"
import { UserPreferencesProvider } from "@/contexts/user-preferences"
import { ServerUnavailableState } from "@/features/session/server-unavailable-state"
import { SessionLoadFailedState } from "@/features/session/session-load-failed-state"
import { useSessionLoader } from "@/features/session/use-session-loader"
import type { WorkspaceOutletContext } from "@/features/workspace/workspace-context"
import { WorkspaceNavigation } from "@/features/workspace/workspace-navigation"

/** 页面导航后清除文字选区。 */
function useClearSelectionOnNavigation() {
  const location = useLocation()

  useLayoutEffect(() => {
    window.getSelection()?.removeAllRanges()
  }, [location.key])
}

/** 读取会话并渲染工作台导航和子页面。 */
export function WorkspaceLayout({
  allowServerChange = false,
}: {
  allowServerChange?: boolean
}) {
  useClearSelectionOnNavigation()
  const { t } = useTranslation("workspace")
  const navigate = useNavigate()
  const [identity, setIdentity] = useState<Identity | null>(null)
  const [loggingOut, setLoggingOut] = useState(false)
  const { status, session, retry } = useSessionLoader()

  useEffect(() => {
    if (status !== "loaded" || !session) {
      return
    }
    if (session.state === SessionState.SessionStateReady && session.identity) {
      setIdentity(session.identity)
      console.info("工作台身份已加载", {
        organization: session.identity.organization.name,
      })
      return
    }
    const path = sessionPath(session.state)
    if (path) {
      navigate(path, { replace: true })
    }
  }, [navigate, session, status])

  /** 退出登录并回到登录页。 */
  async function handleLogout() {
    setLoggingOut(true)
    try {
      await logout()
      console.info("用户退出登录")
    } catch (error) {
      console.warn("退出登录失败", error)
      toast.error(t("logoutError"))
    } finally {
      setLoggingOut(false)
      navigate("/login", { replace: true })
    }
  }

  /** 把保存后的用户资料同步到工作台导航。 */
  function updateUser(user: User) {
    setIdentity((current) => (current ? { ...current, user } : current))
  }

  if (status === "unavailable" && !identity) {
    return (
      <ServerUnavailableState
        onRetry={retry}
        onChangeServer={
          allowServerChange
            ? () => navigate("/connect", { replace: true })
            : undefined
        }
      />
    )
  }

  if (
    !identity &&
    (status === "loading" ||
      (status === "loaded" &&
        session?.state === SessionState.SessionStateReady))
  ) {
    return (
      <main className="flex min-h-svh items-center justify-center gap-2 text-sm text-muted-foreground">
        <LoaderCircleIcon className="size-4 animate-spin" />
        {t("loading")}
      </main>
    )
  }

  if (!identity) {
    return <SessionLoadFailedState onRetry={retry} />
  }

  return (
    <UserPreferencesProvider user={identity.user}>
      <div className="cervi-workspace-shell flex h-svh min-h-0 w-full overflow-hidden">
        <WorkspaceNavigation
          identity={identity}
          onUserUpdated={updateUser}
          onLogout={handleLogout}
          loggingOut={loggingOut}
        />
        <div className="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden bg-background">
          <Outlet
            context={{ identity, updateUser } satisfies WorkspaceOutletContext}
          />
        </div>
      </div>
    </UserPreferencesProvider>
  )
}
