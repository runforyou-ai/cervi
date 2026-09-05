/** 移动端独立入口和路由。 */
import { useEffect } from "react"
import { Navigate, Route, Routes } from "react-router"

import { MobileDirectConversationPage } from "@/apps/mobile/mobile-direct-conversation-page"
import { MobileEmployeeChatPage } from "@/apps/mobile/mobile-employee-chat-page"
import { MobileEmployeeProfilePage } from "@/apps/mobile/mobile-employee-profile-page"
import { MobileEmployeesPage } from "@/apps/mobile/mobile-employees-page"
import { MobileInboxPage } from "@/apps/mobile/mobile-inbox-page"
import { MobileMePage, MobileSettingsPage } from "@/apps/mobile/mobile-me-page"
import {
  MobileContactCategoryPage,
  MobileContactsPage,
} from "@/apps/mobile/mobile-contacts-page"
import {
  MobileDetailLayout,
  MobileTabLayout,
  MobileWorkspaceLayout,
} from "@/apps/mobile/mobile-workspace-layout"
import { LoginPage } from "@/features/auth/login-page"
import { ServerConnectionPage } from "@/features/server-connection/server-connection-page"
import { usePreventPageSelectAll } from "@/hooks/use-prevent-page-select-all"

/** 渲染移动端路由。 */
export default function MobileApp() {
  usePreventPageSelectAll()

  useEffect(() => {
    /** 原生返回键优先关闭当前浮层，再交由 WebView 返回页面。 */
    function dismissOverlay(event: Event) {
      if (
        !document.querySelector(
          '[data-state="open"][role="dialog"], [data-state="open"][role="alertdialog"], [data-state="open"][role="menu"]',
        )
      )
        return
      event.preventDefault()
      document.dispatchEvent(
        new KeyboardEvent("keydown", {
          key: "Escape",
          bubbles: true,
          cancelable: true,
        }),
      )
    }
    window.addEventListener("cervi:back", dismissOverlay, true)
    return () => window.removeEventListener("cervi:back", dismissOverlay, true)
  }, [])
  return (
    <div className="h-dvh w-full overflow-x-hidden">
      <Routes>
        <Route path="/" element={<Navigate to="/inbox" replace />} />
        <Route path="/connect" element={<ServerConnectionPage />} />
        <Route path="/login" element={<LoginPage allowServerChange />} />
        <Route path="/setup" element={<Navigate to="/connect" replace />} />
        <Route element={<MobileWorkspaceLayout />}>
          <Route element={<MobileTabLayout />}>
            <Route path="/inbox" element={<MobileInboxPage />} />
            <Route path="/contacts" element={<MobileContactsPage />} />
            <Route path="/me" element={<MobileMePage />} />
          </Route>
          <Route element={<MobileDetailLayout />}>
            <Route
              path="/inbox/direct/:conversationID"
              element={<MobileDirectConversationPage />}
            />
            <Route path="/me/settings" element={<MobileSettingsPage />} />
            <Route path="/contacts/employees" element={<MobileEmployeesPage />} />
            <Route
              path="/contacts/employees/:userID"
              element={<MobileEmployeeProfilePage />}
            />
            <Route
              path="/contacts/employees/:userID/chat"
              element={<MobileEmployeeChatPage />}
            />
            <Route
              path="/contacts/:category"
              element={<MobileContactCategoryPage />}
            />
          </Route>
        </Route>
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </div>
  )
}
