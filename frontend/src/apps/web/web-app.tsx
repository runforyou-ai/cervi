import { Navigate, Route, Routes } from "react-router"

import { LoginPage } from "@/features/auth/login-page"
import { SetupPage } from "@/features/installation/setup-page"
import { SettingsPage } from "@/features/settings/settings-page"
import {
  WorkspaceInboxPage,
  WorkspacePage,
} from "@/features/workspace/workspace-page"

export default function WebApp() {
  return (
    <Routes>
      <Route path="/" element={<Navigate to="/inbox" replace />} />
      <Route path="/setup" element={<SetupPage />} />
      <Route path="/login" element={<LoginPage />} />
      <Route element={<WorkspacePage />}>
        <Route path="/inbox" element={<WorkspaceInboxPage />} />
        <Route path="/settings" element={<SettingsPage />} />
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}
