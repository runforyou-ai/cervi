/** 移动端独立入口和路由。 */
import { useEffect } from "react"
import { Navigate, Route, Routes } from "react-router"

import { MobileCreateGroupPage } from "@/apps/mobile/mobile-create-group-page"
import { MobileAddGroupMembersPage } from "@/apps/mobile/mobile-add-group-members-page"
import { MobileCustomerBusinessPage } from "@/apps/mobile/mobile-customer-business-page"
import { MobileCustomerConversationPage } from "@/apps/mobile/mobile-customer-conversation-page"
import { MobileServiceCopilotPage } from "@/apps/mobile/mobile-customer-copilot-page"
import { MobileCustomerProfilePage } from "@/apps/mobile/mobile-customer-profile-page"
import { MobileIndividualConversationPage } from "@/apps/mobile/mobile-individual-conversation-page"
import { MobileIndividualProfilePage } from "@/apps/mobile/mobile-individual-profile-page"
import { MobileEmployeeChatPage } from "@/apps/mobile/mobile-employee-chat-page"
import { MobileEmployeeProfilePage } from "@/apps/mobile/mobile-employee-profile-page"
import { MobileGroupConversationPage } from "@/apps/mobile/mobile-group-conversation-page"
import { MobileGroupProfileEditor } from "@/apps/mobile/mobile-group-profile-editor"
import { MobileGroupMembersPage } from "@/apps/mobile/mobile-group-members"
import { MobileGroupMemberActionPage } from "@/apps/mobile/mobile-group-member-management"
import { MobileGroupDetailsPage } from "@/apps/mobile/mobile-group-details-page"
import { MobileDevicesPage } from "@/apps/mobile/mobile-devices-page"
import { MobileDirectoryPage } from "@/apps/mobile/mobile-directory-page"
import { MobileAgentConversationPage } from "@/apps/mobile/mobile-agent-chat-page"
import { MobileChatsPage } from "@/apps/mobile/mobile-chats-page"
import {
  MobileNewChatPage,
  MobileNewChatTargetPage,
} from "@/apps/mobile/mobile-new-chat-page"
import { MobileInboxPage } from "@/apps/mobile/mobile-inbox-page"
import { MobileInboxSearchPage } from "@/apps/mobile/mobile-inbox-search-page"
import {
  MobileMePage,
  MobileMeSettingsPage,
} from "@/apps/mobile/mobile-me-page"
import {
  MobileAssistantEditPage,
  MobileAssistantPage,
  MobileAssistantsPage,
} from "@/apps/mobile/mobile-assistants-page"
import { MobileContactsPage } from "@/apps/mobile/mobile-contacts-page"
import {
  MobileCreateExternalContactPage,
  MobileExternalContactFieldPage,
} from "@/apps/mobile/mobile-external-contact-editor"
import {
  MobileExternalContactPage,
  MobileExternalContactsPage,
} from "@/apps/mobile/mobile-external-contacts-page"
import {
  MobileTeamMembersPage,
  MobileTeamsPage,
} from "@/apps/mobile/mobile-teams-page"
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
            <Route path="/chats" element={<MobileChatsPage />} />
            <Route path="/inbox" element={<MobileInboxPage />} />
            <Route path="/contacts" element={<MobileContactsPage />} />
            <Route path="/me" element={<MobileMePage />} />
          </Route>
          <Route element={<MobileDetailLayout />}>
            <Route path="/search" element={<MobileInboxSearchPage />} />
            <Route
              path="/chats/group/new"
              element={<MobileCreateGroupPage />}
            />
            <Route
              path="/chats/group/:conversationID"
              element={<MobileGroupConversationPage />}
            >
              <Route path="details" element={<MobileGroupDetailsPage />}>
                <Route path="members" element={<MobileGroupMembersPage />} />
                <Route
                  path="add-members"
                  element={<MobileAddGroupMembersPage />}
                />
                <Route
                  path="remove-members"
                  element={
                    <MobileGroupMemberActionPage action="remove" />
                  }
                />
                <Route
                  path="transfer-owner"
                  element={
                    <MobileGroupMemberActionPage action="transfer" />
                  }
                />
                <Route
                  path="edit/:field"
                  element={<MobileGroupProfileEditor />}
                />
              </Route>
            </Route>
            <Route path="/chats/new" element={<MobileNewChatPage />} />
            <Route
              path="/chats/new/:identityID"
              element={<MobileNewChatTargetPage />}
            />
            <Route
              path="/chats/agent/:conversationID"
              element={<MobileAgentConversationPage />}
            >
              <Route path="profile" element={<MobileIndividualProfilePage />} />
            </Route>
            <Route
              path="/inbox/customer/:conversationID"
              element={<MobileCustomerConversationPage />}
            >
              <Route path="copilot" element={<MobileServiceCopilotPage />} />
              <Route path="profile" element={<MobileCustomerProfilePage />} />
              <Route path="business" element={<MobileCustomerBusinessPage />} />
            </Route>
            <Route
              path="/chats/direct/:conversationID"
              element={<MobileIndividualConversationPage />}
            >
              <Route path="profile" element={<MobileIndividualProfilePage />} />
            </Route>
            <Route
              path="/me/profile"
              element={<MobileMeSettingsPage section="profile" />}
            />
            <Route
              path="/me/security"
              element={<MobileMeSettingsPage section="security" />}
            />
            <Route
              path="/me/preferences"
              element={<MobileMeSettingsPage section="preferences" />}
            />
            <Route
              path="/me/notifications"
              element={<MobileMeSettingsPage section="notifications" />}
            />
            <Route path="/me/devices" element={<MobileDevicesPage />} />
            <Route path="/contacts/employees" element={<MobileDirectoryPage />} />
            <Route
              path="/contacts/employees/:userID"
              element={<MobileEmployeeProfilePage />}
            />
            <Route
              path="/contacts/employees/:userID/chat"
              element={<MobileEmployeeChatPage />}
            />
            <Route
              path="/contacts/assistants"
              element={<MobileAssistantsPage />}
            />
            <Route
              path="/contacts/assistants/:assistantID"
              element={<MobileAssistantPage />}
            />
            <Route
              path="/contacts/assistants/:assistantID/edit"
              element={<MobileAssistantEditPage />}
            />
            <Route path="/contacts/teams" element={<MobileTeamsPage />} />
            <Route
              path="/contacts/teams/:teamID"
              element={<MobileTeamMembersPage />}
            />
            <Route
              path="/contacts/external"
              element={<MobileExternalContactsPage />}
            />
            <Route
              path="/contacts/external/new"
              element={<MobileCreateExternalContactPage />}
            />
            <Route
              path="/contacts/external/:contactID"
              element={<MobileExternalContactPage />}
            />
            <Route
              path="/contacts/external/:contactID/edit/:field"
              element={<MobileExternalContactFieldPage />}
            />
          </Route>
        </Route>
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </div>
  )
}
