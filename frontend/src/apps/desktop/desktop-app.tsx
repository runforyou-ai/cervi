import { Navigate, Route, Routes } from "react-router"

import { LoginPage } from "@/features/auth/login-page"
import { SetupPage } from "@/features/installation/setup-page"
import { ServerConnectionPage } from "@/features/server-connection/server-connection-page"
import { WorkspacePage } from "@/features/workspace/workspace-page"

export default function DesktopApp() {
  return (
    <Routes>
      <Route path="/" element={<Navigate to="/inbox" replace />} />
      <Route path="/connect" element={<ServerConnectionPage />} />
      <Route path="/setup" element={<SetupPage />} />
      <Route path="/login" element={<LoginPage />} />
      <Route path="/inbox" element={<WorkspacePage />} />
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}
