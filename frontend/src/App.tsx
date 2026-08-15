import { useState } from "react"
import { Navigate, Route, Routes } from "react-router"

import { LoginPage } from "@/components/login-page"
import { Toaster } from "@/components/ui/sonner"
import { WorkspacePage } from "@/components/workspace-page"

function App() {
  const [isAuthenticated, setIsAuthenticated] = useState(false)
  const workspacePath = "/workspace/inbox"

  return (
    <>
      <Routes>
        <Route
          path="/"
          element={
            <Navigate to={isAuthenticated ? workspacePath : "/login"} replace />
          }
        />
        <Route
          path="/login"
          element={
            isAuthenticated ? (
              <Navigate to={workspacePath} replace />
            ) : (
              <LoginPage onLogin={() => setIsAuthenticated(true)} />
            )
          }
        />
        <Route
          path="/workspace"
          element={
            <Navigate to={isAuthenticated ? workspacePath : "/login"} replace />
          }
        />
        <Route
          path="/workspace/:menuId"
          element={
            isAuthenticated ? (
              <WorkspacePage onLogout={() => setIsAuthenticated(false)} />
            ) : (
              <Navigate to="/login" replace />
            )
          }
        />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
      <Toaster />
    </>
  )
}

export default App
