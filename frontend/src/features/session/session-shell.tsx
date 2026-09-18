/** 登录外壳：读取登录身份并分流会话入口，建立本窗口的实时连接，装配登录期共享上下文。 */
import type { ReactNode } from "react"
import { useTranslation } from "react-i18next"
import { Navigate } from "react-router"

import type { Identity } from "@/api"
import { LoadingIndicator } from "@/components/loading-indicator"
import { RealtimeSyncProvider } from "@/contexts/realtime-sync-context"
import { UserPreferencesProvider } from "@/contexts/user-preferences"
import { AttachmentQueueProvider } from "@/features/inbox/attachment-queue-context"
import { OutgoingMessageProvider } from "@/features/inbox/outgoing-message-context"
import { useIdentityLoader } from "@/features/session/use-identity-loader"
import { useRealtimeConnection } from "@/features/session/use-realtime-connection"

/** 身份就绪后把登录身份交给宿主渲染，未登录或需要切换入口时跳转。 */
export function SessionShell({
  children,
}: {
  children: (identity: Identity) => ReactNode
}) {
  const { t } = useTranslation(["workspace", "common"])
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
            {children(identity)}
          </AttachmentQueueProvider>
        </RealtimeSyncProvider>
      </OutgoingMessageProvider>
    </UserPreferencesProvider>
  )
}
