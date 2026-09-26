/** Web 与桌面端共用的业务路由。 */
import { lazy } from "react"
import { Navigate, Route, Routes } from "react-router"

import { LoginPage } from "@/features/auth/login-page"
import { OfficialLoginCallbackPage } from "@/features/auth/official-login-callback-page"
import { InvalidAddressPage } from "@/features/installation/invalid-address-page"
import { SetupPage } from "@/features/installation/setup-page"
import { ServerConnectionPage } from "@/features/server-connection/server-connection-page"
import { usePreventPageSelectAll } from "@/hooks/use-prevent-page-select-all"
import { useSessionGeneration } from "@/hooks/use-session-generation"

// 工作台与会话独立窗口在首次进入时加载，入口页不携带工作台代码。
const WorkspaceLayout = lazy(() =>
  import("@/features/workspace/workspace-layout").then((module) => ({ default: module.WorkspaceLayout })),
)
const ConversationWindowLayout = lazy(() =>
  import("@/features/workspace/conversation-window-layout").then((module) => ({ default: module.ConversationWindowLayout })),
)

/** 按平台注册入口页面，并把工作台页面交给标签宿主管理；桌面端另有会话独立窗口路由；登录会话代次变化时重新挂载登录外壳。 */
export function SharedAppRoutes({ platform }: { platform: "web" | "desktop" }) {
  usePreventPageSelectAll()
  const sessionGeneration = useSessionGeneration()

  return (
    <Routes>
      <Route path="/" element={<Navigate to="/inbox" replace />} />
      {platform === "desktop" ? (
        <Route path="/connect" element={<ServerConnectionPage />} />
      ) : null}
      {platform === "desktop" ? (
        <Route
          path="/conversations/:conversationId"
          element={<ConversationWindowLayout key={sessionGeneration} />}
        />
      ) : null}
      {platform === "web" ? (
        <Route path="/setup" element={<SetupPage />} />
      ) : null}
      {platform === "web" ? (
        <Route path="/invalid-address" element={<InvalidAddressPage />} />
      ) : null}
      <Route
        path="/login"
        element={<LoginPage allowServerChange={platform === "desktop"} />}
      />
      {platform === "web" ? (
        <Route path="/auth/callback" element={<OfficialLoginCallbackPage />} />
      ) : null}
      <Route path="*" element={<WorkspaceLayout key={sessionGeneration} />} />
    </Routes>
  )
}
