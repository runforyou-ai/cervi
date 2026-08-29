/** 提供工作台单页面模式。 */
import type { WorkspaceOutletContext } from "@/contexts/workspace-context"
import { WorkspaceProvider } from "@/contexts/workspace-context"
import { WorkspacePageRoutes } from "@/features/workspace/workspace-page-routes"

/** 渲染当前工作台地址。 */
export function WorkspaceSinglePage({
  href,
  context,
}: {
  href: string
  context: WorkspaceOutletContext
}) {
  return (
    <div className="relative flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden bg-background">
      <WorkspaceProvider value={context}>
        <WorkspacePageRoutes location={href} />
      </WorkspaceProvider>
    </div>
  )
}
