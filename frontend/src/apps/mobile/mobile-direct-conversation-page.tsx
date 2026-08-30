/** 移动端企业成员内部单聊详情。 */
import { useMemo, useState } from "react"
import { ArrowLeftIcon, LoaderCircleIcon, UserRoundIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import {
  Navigate,
  useLocation,
  useNavigate,
  useParams,
} from "react-router"

import {
  ConversationType,
  isDirectInboxConversation,
  loadInbox,
  type ConversationMessage,
  type DirectInboxConversationData,
  type InboxConversation,
} from "@/api"
import { useMobileWorkspace } from "@/apps/mobile/mobile-workspace-layout"
import { Button } from "@/components/ui/button"
import { ConversationComposer } from "@/features/inbox/conversation-composer"
import { ConversationTimeline } from "@/features/inbox/conversation-timeline"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"

type MobileDirectLocationState = {
  conversation?: InboxConversation
}

/** 从路由状态读取本次打开的 Direct 摘要。 */
function directConversationFromState(
  state: unknown,
  conversationID: string,
): DirectInboxConversationData | null {
  const candidate = (state as MobileDirectLocationState | null)?.conversation
  return candidate?.id === conversationID &&
    isDirectInboxConversation(candidate)
    ? candidate
    : null
}

/** 展示当前 Direct 的移动端头部。 */
function MobileDirectHeader({ peerName }: { peerName: string }) {
  const { t } = useTranslation("common")
  const navigate = useNavigate()
  const initial = Array.from(peerName)[0]?.toLocaleUpperCase()

  return (
    <header className="flex h-14 shrink-0 items-center gap-3 border-b px-2">
      <Button
        type="button"
        variant="ghost"
        size="icon-lg"
        aria-label={t("actions.back")}
        onClick={() => navigate("/inbox", { replace: true })}
      >
        <ArrowLeftIcon />
      </Button>
      <span className="flex size-9 shrink-0 items-center justify-center rounded-full bg-muted text-sm font-medium text-muted-foreground">
        {initial ? initial : <UserRoundIcon className="size-4" />}
      </span>
      <h1 className="min-w-0 truncate text-base font-semibold">{peerName}</h1>
    </header>
  )
}

/** 加载并显示移动端 Direct 历史和文本发送区。 */
export function MobileDirectConversationPage() {
  const { t } = useTranslation("inbox")
  const { identity } = useMobileWorkspace()
  const location = useLocation()
  const { conversationID = "" } = useParams()
  const stateConversation = useMemo(
    () => directConversationFromState(location.state, conversationID),
    [conversationID, location.state],
  )
  const { data, loading } = useResource(
    resourceKeys.inbox(),
    () => loadInbox(),
    {
      enabled: stateConversation === null,
      staleTime: 0,
      refetchOnWindowFocus: false,
    },
  )
  const [sentMessages, setSentMessages] = useState<ConversationMessage[]>([])

  if (!conversationID) return <Navigate to="/inbox" replace />

  const matchedConversation = data?.conversations.find(
    (conversation) => conversation.id === conversationID,
  )
  if (matchedConversation && !isDirectInboxConversation(matchedConversation)) {
    return <Navigate to="/inbox" replace />
  }
  const conversation =
    stateConversation ??
    (matchedConversation && isDirectInboxConversation(matchedConversation)
      ? matchedConversation
      : null)
  const peerName = conversation?.direct.peerName.trim() || t("unknownSender")

  return (
    <section className="flex h-full min-h-0 flex-col bg-background">
      <MobileDirectHeader peerName={peerName} />
      {loading && !conversation ? (
        <div className="flex min-h-0 flex-1 items-center justify-center gap-2 text-sm text-muted-foreground">
          <LoaderCircleIcon className="size-4 animate-spin" />
          {t("messagesLoading")}
        </div>
      ) : (
        <>
          <ConversationTimeline
            conversationID={conversationID}
            conversationType={ConversationType.ConversationTypeDirect}
            currentIdentityID={identity.user.identityId}
            requireWindowFocus={false}
            sentMessages={sentMessages}
          />
          <ConversationComposer
            conversationID={conversationID}
            conversationType={ConversationType.ConversationTypeDirect}
            onSent={(message) =>
              setSentMessages((current) =>
                current.some((item) => item.id === message.id)
                  ? current
                  : [...current, message],
              )
            }
          />
        </>
      )}
    </section>
  )
}
