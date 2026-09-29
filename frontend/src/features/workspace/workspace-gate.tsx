/** 工作区入口：按地址中的工作区标识确定请求目标工作区，账号不在该工作区时回到工作区列表。 */
import { useEffect, useLayoutEffect, useState, type ReactNode } from "react"
import { useTranslation } from "react-i18next"

import { listWorkspaces } from "@/api"
import { setRequestWorkspace } from "@/api/client"
import { LoadingIndicator } from "@/components/loading-indicator"
import { PageLoadError } from "@/components/page-load-error"
import { WorkspaceScopeProvider } from "@/contexts/workspace-scope-context"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"
import { navigateToHashPath, rememberWorkspaceSlug } from "@/lib/workspace-route"

/** 找到地址对应的工作区后设置请求目标并渲染工作区页面。 */
export function WorkspaceGate({ slug, children }: { slug: string; children: ReactNode }) {
  const { t } = useTranslation(["account", "common"])
  const { data, error, retrying, refresh } = useResource(resourceKeys.workspaces(), (signal) => listWorkspaces(signal))
  const workspace = data?.items.find((item) => item.slug === slug)
  const [readyID, setReadyID] = useState("")

  // 工作区确定后先设置请求目标，再渲染发起请求的页面。
  useLayoutEffect(() => {
    if (!workspace) return
    setRequestWorkspace(workspace.id)
    rememberWorkspaceSlug(workspace.slug)
    setReadyID(workspace.id)
    return () => setRequestWorkspace("")
  }, [workspace])

  // 账号不在地址对应的工作区时回到工作区列表。
  useEffect(() => {
    if (data && !workspace) {
      console.info("地址中的工作区不可进入", { slug })
      navigateToHashPath("/workspaces", { replace: true })
    }
  }, [data, workspace, slug])

  if (error && !data && !retrying) {
    return <PageLoadError message={t("account:loadError")} onRetry={refresh} />
  }
  if (!data || !workspace || readyID !== workspace.id) {
    return (
      <main className="flex min-h-svh items-center justify-center">
        <LoadingIndicator>{t("common:status.loading")}</LoadingIndicator>
      </main>
    )
  }
  return (
    <WorkspaceScopeProvider value={{ current: workspace, workspaces: data.items }}>
      {children}
    </WorkspaceScopeProvider>
  )
}
