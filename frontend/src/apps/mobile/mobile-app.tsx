/** 移动端独立入口和路由。 */
import { Navigate, Route, Routes } from "react-router"

import { MobileInboxPage } from "@/apps/mobile/mobile-inbox-page"
import { MobileMePage } from "@/apps/mobile/mobile-me-page"
import { MobileWorkspaceLayout } from "@/apps/mobile/mobile-workspace-layout"
import { LoginPage } from "@/features/auth/login-page"
import { ServerConnectionPage } from "@/features/server-connection/server-connection-page"

/** 渲染移动端路由。 */
export default function MobileApp() {
  return (
    <Routes>
      <Route path="/" element={<Navigate to="/inbox" replace />} />
      <Route path="/connect" element={<ServerConnectionPage />} />
      <Route path="/login" element={<LoginPage allowServerChange />} />
      <Route path="/setup" element={<Navigate to="/connect" replace />} />
      <Route element={<MobileWorkspaceLayout />}>
        <Route path="/inbox" element={<MobileInboxPage />} />
        <Route path="/me" element={<MobileMePage />} />
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}
