/** Web 与桌面端共用的业务路由。 */
import { Navigate, Route, Routes } from "react-router"

import { LoginPage } from "@/features/auth/login-page"
import { MessageChannelListPage } from "@/features/channels/message-channel-list-page"
import { WebsiteChannelFormPage } from "@/features/channels/website/website-channel-form-page"
import { ContactsPage } from "@/features/contacts/contacts-page"
import { InboxRoute } from "@/features/inbox/inbox-route"
import { IntegrationsLayout } from "@/features/integrations/integrations-layout"
import { ModelProviderFormPage } from "@/features/integrations/model-services/model-provider-form-page"
import { ModelProviderListPage } from "@/features/integrations/model-services/model-provider-list-page"
import { SetupPage } from "@/features/installation/setup-page"
import { ServerConnectionPage } from "@/features/server-connection/server-connection-page"
import { RoleFormPage } from "@/features/settings/role-form-page"
import {
  PersonalSettingsPage,
  SystemSettingsPage,
} from "@/features/settings/settings-page"
import { WorkspaceLayout } from "@/features/workspace/workspace-layout"

/** 按平台注册登录、工作台和业务页面路由。 */
export function SharedAppRoutes({ platform }: { platform: "web" | "desktop" }) {
  return (
    <Routes>
      <Route path="/" element={<Navigate to="/inbox" replace />} />
      {platform === "desktop" ? (
        <Route path="/connect" element={<ServerConnectionPage />} />
      ) : null}
      {platform === "web" ? (
        <Route path="/setup" element={<SetupPage />} />
      ) : null}
      <Route
        path="/login"
        element={<LoginPage allowServerChange={platform === "desktop"} />}
      />
      <Route
        element={<WorkspaceLayout allowServerChange={platform === "desktop"} />}
      >
        <Route path="/inbox" element={<InboxRoute />} />
        <Route
          path="/account"
          element={<Navigate to="/account/profile" replace />}
        />
        <Route
          path="/account/profile"
          element={<PersonalSettingsPage section="profile" />}
        />
        <Route
          path="/account/security"
          element={<PersonalSettingsPage section="security" />}
        />
        <Route
          path="/account/preferences"
          element={<PersonalSettingsPage section="preferences" />}
        />
        <Route
          path="/settings"
          element={<Navigate to="/settings/organization" replace />}
        />
        <Route
          path="/settings/organization"
          element={<SystemSettingsPage section="organization" />}
        />
        <Route
          path="/settings/roles"
          element={<SystemSettingsPage section="roles" />}
        />
        <Route
          path="/settings/roles/new"
          element={
            <SystemSettingsPage section="roles">
              <RoleFormPage mode="create" />
            </SystemSettingsPage>
          }
        />
        <Route
          path="/settings/roles/:roleId"
          element={
            <SystemSettingsPage section="roles">
              <RoleFormPage mode="detail" />
            </SystemSettingsPage>
          }
        />
        <Route
          path="/settings/storage"
          element={<SystemSettingsPage section="storage" />}
        />
        <Route
          path="/contacts"
          element={<Navigate to="/contacts/members" replace />}
        />
        <Route
          path="/contacts/members"
          element={<ContactsPage scope="members" />}
        />
        <Route
          path="/contacts/external"
          element={<ContactsPage scope="external" />}
        />
        <Route path="/integrations" element={<IntegrationsLayout />}>
          <Route index element={<Navigate to="channels" replace />} />
          <Route path="channels" element={<MessageChannelListPage />} />
          <Route
            path="channels/new"
            element={<WebsiteChannelFormPage mode="create" />}
          />
          <Route
            path="channels/:channelId"
            element={<WebsiteChannelFormPage mode="edit" />}
          />
          <Route
            path="model-services"
            element={
              <Navigate to="/integrations/model-services/chat" replace />
            }
          />
          <Route
            path="model-services/chat"
            element={<ModelProviderListPage section="chat" />}
          />
          <Route
            path="model-services/chat/new"
            element={
              <ModelProviderFormPage mode="create" returnSection="chat" />
            }
          />
          <Route
            path="model-services/chat/:providerId"
            element={
              <ModelProviderFormPage mode="edit" returnSection="chat" />
            }
          />
          <Route
            path="model-services/embedding"
            element={<ModelProviderListPage section="embedding" />}
          />
          <Route
            path="model-services/embedding/new"
            element={
              <ModelProviderFormPage mode="create" returnSection="embedding" />
            }
          />
          <Route
            path="model-services/embedding/:providerId"
            element={
              <ModelProviderFormPage mode="edit" returnSection="embedding" />
            }
          />
          <Route
            path="model-services/rerank"
            element={<ModelProviderListPage section="rerank" />}
          />
          <Route
            path="model-services/rerank/new"
            element={
              <ModelProviderFormPage mode="create" returnSection="rerank" />
            }
          />
          <Route
            path="model-services/rerank/:providerId"
            element={
              <ModelProviderFormPage mode="edit" returnSection="rerank" />
            }
          />
        </Route>
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}
