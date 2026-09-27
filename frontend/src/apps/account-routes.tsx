/** 根路径下的账号级路由与工作区路由器内的转交路由。 */
import { useEffect, type ReactNode } from "react"
import { Navigate, Route, Routes, useLocation } from "react-router"

import { RegisterPage } from "@/features/account/register-page"
import { WorkspaceEntry } from "@/features/account/workspace-entry"
import { WorkspaceListPage } from "@/features/account/workspace-list-page"
import { LoginPage } from "@/features/auth/login-page"
import { OfficialLoginCallbackPage } from "@/features/auth/official-login-callback-page"
import { SetupPage } from "@/features/installation/setup-page"
import { ServerConnectionPage } from "@/features/server-connection/server-connection-page"
import { WorkspaceGate } from "@/features/workspace/workspace-gate"
import { navigateToHashPath } from "@/lib/workspace-route"
import type { AppPlatform } from "@/platform/app-platform"

// 账号级页面的路径，工作区路由器内出现这些地址时转交根路由器。
const accountPaths = ["/login", "/register", "/setup", "/connect", "/workspaces", "/auth/callback"]

/** 渲染登录、注册、首次安装、服务器连接和工作区列表；其余地址进入最近使用的工作区。 */
export function AccountRoutes({ platform }: { platform: AppPlatform }) {
  const native = platform !== "web"
  return (
    <Routes>
      <Route path="/login" element={<LoginPage allowServerChange={native} />} />
      <Route path="/register" element={<RegisterPage />} />
      <Route path="/setup" element={native ? <Navigate to="/connect" replace /> : <SetupPage />} />
      {native ? <Route path="/connect" element={<ServerConnectionPage />} /> : null}
      {native ? null : <Route path="/auth/callback" element={<OfficialLoginCallbackPage />} />}
      <Route path="/workspaces" element={<WorkspaceListPage />} />
      <Route path="*" element={<WorkspaceEntry />} />
    </Routes>
  )
}

/** 把工作区路由器内的账号级地址交给根路由器。 */
function LeaveWorkspace() {
  const location = useLocation()
  useEffect(() => {
    navigateToHashPath(`${location.pathname}${location.search}`, { replace: true })
  }, [location.pathname, location.search])
  return null
}

/** 工作区路由器：账号级地址转交根路由器，其余地址在确定工作区后渲染工作区页面。 */
export function WorkspaceRoutes({ slug, children }: { slug: string; children: ReactNode }) {
  return (
    <Routes>
      {accountPaths.map((path) => (
        <Route key={path} path={path} element={<LeaveWorkspace />} />
      ))}
      <Route path="*" element={<WorkspaceGate slug={slug}>{children}</WorkspaceGate>} />
    </Routes>
  )
}
