/** Web 与桌面端共用的业务路由。 */
import { Navigate, Route, Routes } from "react-router"

import { LoginPage } from "@/features/auth/login-page"
import { SetupPage } from "@/features/installation/setup-page"
import { ServerConnectionPage } from "@/features/server-connection/server-connection-page"
import { WorkspaceLayout } from "@/features/workspace/workspace-layout"

/** 按平台注册入口页面，并把工作台页面交给标签宿主管理。 */
export function SharedAppRoutes({ platform }: { platform: "web" | "desktop" }) {
  return (
    <Routes>
      <Route path="/" element={<Navigate to="/inbox" replace />} />
      {platform === "desktop" ? (
        <Route path="/connect" element={<ServerConnectionPage />} />
      ) : null}
      {platform === "web" ? (
        <Route path="/setup" element={<SetupPage />} />
      ) : null}
      <Route
        path="/login"
        element={<LoginPage allowServerChange={platform === "desktop"} />}
      />
      <Route path="*" element={<WorkspaceLayout />} />
    </Routes>
  )
}
