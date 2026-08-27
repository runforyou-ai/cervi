/** 移动端独立入口和路由。 */
import { Navigate, Route, Routes } from "react-router"

import { MobileHomePage } from "@/apps/mobile/mobile-home-page"
import { LoginPage } from "@/features/auth/login-page"
import { ServerConnectionPage } from "@/features/server-connection/server-connection-page"

/** 渲染移动端路由。 */
export default function MobileApp() {
  return (
    <Routes>
      <Route path="/" element={<Navigate to="/inbox" replace />} />
      <Route path="/connect" element={<ServerConnectionPage />} />
      <Route path="/login" element={<LoginPage allowServerChange />} />
      <Route path="/inbox" element={<MobileHomePage />} />
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}
