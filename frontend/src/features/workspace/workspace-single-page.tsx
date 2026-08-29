/** 在关闭多标签页时渲染单个工作台页面。 */
import type { WorkspaceOutletContext } from "@/contexts/workspace-context"
import { WorkspaceProvider } from "@/contexts/workspace-context"
import {
  WorkspacePageRoutes,
  type ResolvedWorkspaceTab,
} from "@/features/workspace/workspace-page-routes"

/** 使用当前地址渲染唯一的工作台页面实例。 */
export function WorkspaceSinglePage({
  currentTab,
  context,
}: {
  currentTab: ResolvedWorkspaceTab
  context: WorkspaceOutletContext
}) {
  return (
    <div className="relative flex min-h-0 min-w-0 flex-1 overflow-hidden bg-background">
      <div className="flex h-full min-h-0 w-full flex-col overflow-hidden">
        <WorkspaceProvider value={context}>
          <WorkspacePageRoutes location={currentTab.href} />
        </WorkspaceProvider>
      </div>
    </div>
  )
}
