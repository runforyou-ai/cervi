/** 桌面端会话独立窗口的登录外壳。 */
import { useTranslation } from "react-i18next"
import { Navigate, useParams } from "react-router"

import { LoadingIndicator } from "@/components/loading-indicator"
import { RealtimeSyncProvider } from "@/contexts/realtime-sync-context"
import { UserPreferencesProvider } from "@/contexts/user-preferences"
import { WorkspaceProvider } from "@/contexts/workspace-context"
import { AttachmentQueueProvider } from "@/features/inbox/attachment-queue-context"
import { ConversationWindowPage } from "@/features/inbox/conversation-window-page"
import { OutgoingMessageProvider } from "@/features/inbox/outgoing-message-context"
import { useIdentityLoader } from "@/features/session/use-identity-loader"
import { useRealtimeConnection } from "@/features/session/use-realtime-connection"

/** 读取登录身份、建立本窗口的实时连接，并渲染单个会话。 */
export function ConversationWindowLayout() {
  const { t } = useTranslation(["workspace", "common"])
  const { conversationId = "" } = useParams()
  const { status, identity, redirectPath } = useIdentityLoader()
  useRealtimeConnection(Boolean(identity?.user.id))

  if (status === "anonymous") return <Navigate to="/login" replace />
  if (status === "redirect" && redirectPath) {
    return <Navigate to={redirectPath} replace />
  }
  if (status === "failed") {
    return (
      <main className="flex min-h-svh items-center justify-center text-sm text-muted-foreground">
        {t("identityLoadError")}
      </main>
    )
  }
  if (!identity) {
    return (
      <main className="flex min-h-svh items-center justify-center">
        <LoadingIndicator>{t("common:status.loading")}</LoadingIndicator>
      </main>
    )
  }

  return (
    <UserPreferencesProvider user={identity.user}>
      <OutgoingMessageProvider key={identity.user.id}>
        <RealtimeSyncProvider>
          <AttachmentQueueProvider key={identity.user.id}>
            <WorkspaceProvider value={{ identity }}>
              <div className="cervi-conversation-window flex h-svh min-h-0 w-full flex-col overflow-hidden bg-background">
                <ConversationWindowPage
                  key={conversationId}
                  conversationId={conversationId}
                />
              </div>
            </WorkspaceProvider>
          </AttachmentQueueProvider>
        </RealtimeSyncProvider>
      </OutgoingMessageProvider>
    </UserPreferencesProvider>
  )
}
