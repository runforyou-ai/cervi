/** Web 与桌面端工作台布局。 */
import { useLayoutEffect, useState } from "react"
import { useTranslation } from "react-i18next"
import { Outlet, useLocation } from "react-router"
import { toast } from "sonner"

import {
  logout,
  type Identity,
  type Organization,
  type CurrentUser,
} from "@/api"
import { UserPreferencesProvider } from "@/contexts/user-preferences"
import {
  useSessionController,
  useSessionSnapshot,
} from "@/features/session/session-context"
import type { WorkspaceOutletContext } from "@/features/workspace/workspace-context"
import { WorkspaceNavigation } from "@/features/workspace/workspace-navigation"

/** 页面导航后清除文字选区。 */
function useClearSelectionOnNavigation() {
  const location = useLocation()

  useLayoutEffect(() => {
    window.getSelection()?.removeAllRanges()
  }, [location.key])
}

/** 使用 Gate 已确认的身份渲染工作台导航和子页面。 */
export function WorkspaceLayout() {
  useClearSelectionOnNavigation()
  const { t } = useTranslation("workspace")
  const controller = useSessionController()
  const { session } = useSessionSnapshot()
  const [identity, setIdentity] = useState<Identity>(session!.identity!)
  const [loggingOut, setLoggingOut] = useState(false)

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
      await controller.reload("logout")
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
