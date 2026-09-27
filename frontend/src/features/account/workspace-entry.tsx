/** 根路径下的工作区地址：有待处理的邀请时回到邀请页，否则进入最近使用或唯一的工作区并保留页面路径，都没有时前往工作区列表。 */
import { useEffect } from "react"
import { useTranslation } from "react-i18next"
import { useLocation, useNavigate } from "react-router"

import { listWorkspaces } from "@/api"
import { LoadingIndicator } from "@/components/loading-indicator"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"
import { invitationPath, takePendingInvitation } from "@/lib/pending-invitation"
import { enterWorkspace, lastWorkspaceSlug } from "@/lib/workspace-route"

/** 读取账号可进入的工作区后决定进入哪个工作区。 */
export function WorkspaceEntry() {
  const { t } = useTranslation(["workspace", "common"])
  const location = useLocation()
  const navigate = useNavigate()
  const { data, error } = useResource(resourceKeys.workspaces(), (signal) => listWorkspaces(signal), { staleTime: 0 })

  useEffect(() => {
    if (!data) return
    const invitation = location.pathname === "/" ? takePendingInvitation() : null
    if (invitation) {
      navigate(invitationPath(invitation), { replace: true })
      return
    }
    const lastSlug = lastWorkspaceSlug()
    const target =
      data.items.find((workspace) => workspace.slug === lastSlug) ??
      (data.items.length === 1 ? data.items[0] : undefined)
    if (!target) {
      navigate("/workspaces", { replace: true })
      return
    }
    const path = location.pathname === "/" ? "/inbox" : `${location.pathname}${location.search}`
    enterWorkspace(target.slug, path, { replace: true })
  }, [data, location.pathname, location.search, navigate])

  if (error && !data) {
    return (
      <main className="flex min-h-svh items-center justify-center px-6 text-center text-sm text-muted-foreground">
        {t("identityLoadError")}
      </main>
    )
  }
  return (
    <main className="flex min-h-svh items-center justify-center">
      <LoadingIndicator>{t("common:status.loading")}</LoadingIndicator>
    </main>
  )
}
