/** Web 与桌面端共用的业务路由。 */
import { Navigate, Route, Routes } from "react-router"

import { LoginPage } from "@/features/auth/login-page"
import { SetupPage } from "@/features/installation/setup-page"
import { ServerConnectionPage } from "@/features/server-connection/server-connection-page"
import { ConversationWindowLayout } from "@/features/workspace/conversation-window-layout"
import { WorkspaceLayout } from "@/features/workspace/workspace-layout"
import { usePreventPageSelectAll } from "@/hooks/use-prevent-page-select-all"
import { useSessionGeneration } from "@/hooks/use-session-generation"

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
      <Route
        path="/login"
        element={<LoginPage allowServerChange={platform === "desktop"} />}
      />
      <Route path="*" element={<WorkspaceLayout key={sessionGeneration} />} />
    </Routes>
  )
}
