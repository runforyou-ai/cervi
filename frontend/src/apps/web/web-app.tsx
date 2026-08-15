import { Navigate, Route, Routes } from "react-router"

import { LoginPage } from "@/features/auth/login-page"
import { SetupPage } from "@/features/installation/setup-page"
import { WorkspacePage } from "@/features/workspace/workspace-page"

export default function WebApp() {
  return (
    <Routes>
      <Route path="/" element={<Navigate to="/inbox" replace />} />
      <Route path="/setup" element={<SetupPage />} />
      <Route path="/login" element={<LoginPage />} />
      <Route path="/inbox" element={<WorkspacePage />} />
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}
