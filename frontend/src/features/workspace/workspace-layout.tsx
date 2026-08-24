/** Web 与桌面端工作台布局。 */
import { useEffect, useLayoutEffect, useState } from "react"
import { LoaderCircleIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { Navigate, Outlet, useLocation, useNavigate } from "react-router"
import { toast } from "sonner"

import {
  logout,
  type Identity,
  type Organization,
  type CurrentUser,
} from "@/api"
import { UserPreferencesProvider } from "@/contexts/user-preferences"
import { useIdentityLoader } from "@/features/session/use-identity-loader"
import type { WorkspaceOutletContext } from "@/features/workspace/workspace-context"
import { WorkspaceNavigation } from "@/features/workspace/workspace-navigation"

/** 页面导航后清除文字选区。 */
function useClearSelectionOnNavigation() {
  const location = useLocation()

  useLayoutEffect(() => {
    window.getSelection()?.removeAllRanges()
  }, [location.key])
}

/** 读取登录身份并渲染工作台导航和子页面。 */
export function WorkspaceLayout() {
  useClearSelectionOnNavigation()
  const { t } = useTranslation("workspace")
  const navigate = useNavigate()
  const [identity, setIdentity] = useState<Identity | null>(null)
  const [loggingOut, setLoggingOut] = useState(false)
  const { status, identity: loadedIdentity, redirectPath } = useIdentityLoader()

  /** 身份加载完成后同步工作台状态。 */
  useEffect(() => {
    if (loadedIdentity) {
      setIdentity(loadedIdentity)
      console.info("工作台身份已加载", {
        organization: loadedIdentity.organization.name,
      })
    }
  }, [loadedIdentity])

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
  function updateUser(user: CurrentUser) {
    setIdentity((current) => (current ? { ...current, user } : current))
  }

  /** 同步工作台中的最新企业信息。 */
  function updateOrganization(organization: Organization) {
    setIdentity((current) => (current ? { ...current, organization } : current))
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
      <main className="flex min-h-svh items-center justify-center gap-2 text-sm text-muted-foreground">
        <LoaderCircleIcon className="size-4 animate-spin" />
        {t("loading")}
      </main>
    )
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
            context={{
              identity,
              updateOrganization,
              updateUser,
            } satisfies WorkspaceOutletContext}
          />
        </div>
      </div>
    </UserPreferencesProvider>
  )
}
