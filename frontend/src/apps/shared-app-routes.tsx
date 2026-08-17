import { Navigate, Route, Routes } from "react-router"

import { LoginPage } from "@/features/auth/login-page"
import { WebsiteChannelFormPage } from "@/features/channels/website/website-channel-form-page"
import { WebsiteChannelListPage } from "@/features/channels/website/website-channel-list-page"
import { WebsiteChannelTrashPage } from "@/features/channels/website/website-channel-trash-page"
import { InboxRoute } from "@/features/inbox/inbox-route"
import { SetupPage } from "@/features/installation/setup-page"
import { ServerConnectionPage } from "@/features/server-connection/server-connection-page"
import { SettingsPage } from "@/features/settings/settings-page"
import { WorkspaceLayout } from "@/features/workspace/workspace-layout"

export function SharedAppRoutes({
  includeServerConnection = false,
}: {
  includeServerConnection?: boolean
}) {
  return (
    <Routes>
      <Route path="/" element={<Navigate to="/inbox" replace />} />
      {includeServerConnection ? (
        <Route path="/connect" element={<ServerConnectionPage />} />
      ) : null}
      <Route path="/setup" element={<SetupPage />} />
      <Route path="/login" element={<LoginPage />} />
      <Route element={<WorkspaceLayout />}>
        <Route path="/inbox" element={<InboxRoute />} />
        <Route
          path="/settings"
          element={<Navigate to="/settings/storage" replace />}
        />
        <Route path="/settings/storage" element={<SettingsPage />} />
        <Route
          path="/channels"
          element={<Navigate to="/channels/website" replace />}
        />
        <Route
          path="/channels/website"
          element={<WebsiteChannelListPage />}
        />
        <Route
          path="/channels/website/new"
          element={<WebsiteChannelFormPage mode="create" />}
        />
        <Route
          path="/channels/website/trash"
          element={<WebsiteChannelTrashPage />}
        />
        <Route
          path="/channels/website/:channelId"
          element={<WebsiteChannelFormPage mode="edit" />}
        />
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}
