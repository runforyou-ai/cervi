/** 从成员资料进入已有单聊或尚未发送的草稿。 */
import { useEffect, useRef, useState } from "react"
import { useTranslation } from "react-i18next"
import { Navigate, useLocation, useNavigate, useParams } from "react-router"

import {
  findDirectConversation,
  getUser,
  sendFirstDirectTextMessage,
  UserStatus,
  type UserData,
} from "@/api"
import { MobileDirectThread } from "@/apps/mobile/mobile-direct-thread"
import { MobilePageHeader, MobilePageState } from "@/apps/mobile/mobile-page"
import { useMobileWorkspace } from "@/apps/mobile/mobile-workspace-layout"
import { LoadingIndicator } from "@/components/loading-indicator"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource, useResourceInvalidator } from "@/hooks/use-resource"

/** 按成员隔离草稿生命周期，旧发送结果不改变新页面。 */
export function MobileEmployeeChatPage() {
  const { userID = "" } = useParams()
  return <MobileEmployeeChat key={userID} userID={userID} />
}

/** 草稿建立后卸载入口查询，网络重连和缓存刷新不再替换输入区。 */
function MobileEmployeeChat({ userID }: { userID: string }) {
  const [draftUser, setDraftUser] = useState<UserData | null>(null)
  if (!draftUser) {
    return <MobileEmployeeChatLookup userID={userID} onDraft={setDraftUser} />
  }
  return (
    <section className="flex h-full min-h-0 flex-col">
      <MobilePageHeader
        title={draftUser.displayName}
        backTo={`/contacts/employees/${userID}`}
      />
      <MobileEmployeeDraft user={draftUser} />
    </section>
  )
}

/** 确认成员有效并只读查找已有单聊，首次发送前不创建会话。 */
function MobileEmployeeChatLookup({
  userID,
  onDraft,
}: {
  userID: string
  onDraft: (user: UserData) => void
}) {
  const { t } = useTranslation("mobile")
  const { identity } = useMobileWorkspace()
  const location = useLocation()
  const profileURL = `/contacts/employees/${userID}`
  const member = useResource(resourceKeys.user(userID), () => getUser(userID), {
    staleTime: 0,
  })
  const user = member.data
  const canSend =
    user?.status === UserStatus.UserStatusActive &&
    user.identityId !== identity.user.identityId
  const lookup = useResource(
    resourceKeys.directConversation(user?.identityId ?? ""),
    () => findDirectConversation(user?.identityId ?? ""),
    {
      enabled: canSend && !member.error,
      staleTime: 0,
      refetchOnWindowFocus: false,
    },
  )

  useEffect(() => {
    // 只在本次成员校验和会话查找完成后交接草稿，发送资格仍由服务端校验。
    if (
      user &&
      canSend &&
      !member.error &&
      !member.refreshing &&
      !lookup.error &&
      !lookup.loading &&
      !lookup.refreshing &&
      lookup.data === null
    ) {
      onDraft(user)
    }
  }, [
    user,
    canSend,
    member.error,
    member.refreshing,
    lookup.error,
    lookup.loading,
    lookup.refreshing,
    lookup.data,
    onDraft,
  ])

  if (
    canSend &&
    !member.error &&
    !member.refreshing &&
    !lookup.error &&
    lookup.data &&
    !lookup.refreshing
  ) {
    return (
      <Navigate
        to={`/inbox/direct/${lookup.data.id}`}
        replace
        state={{
          ...location.state,
          conversation: lookup.data,
          memberUserID: userID,
        }}
      />
    )
  }

  return (
    <section className="flex h-full min-h-0 flex-col">
      <MobilePageHeader
        title={user?.displayName ?? t("contacts.sendMessage")}
        backTo={profileURL}
      />
      {member.error || lookup.error ? (
        <MobilePageState
          title={t("contacts.chatError")}
          onRetry={() =>
            void (member.error ? member.refresh() : lookup.refresh())
          }
        />
      ) : user && !canSend ? (
        <MobilePageState title={t("contacts.chatUnavailable")} />
      ) : (
        <LoadingIndicator className="min-h-0 flex-1 justify-center">
          {t("loading")}
        </LoadingIndicator>
      )}
    </section>
  )
}

/** 首次发送成功后用正式会话替换草稿路由，保留成员资料作为返回位置。 */
function MobileEmployeeDraft({ user }: { user: UserData }) {
  const navigate = useNavigate()
  const location = useLocation()
  const invalidate = useResourceInvalidator()
  const alive = useRef(true)
  useEffect(() => {
    alive.current = true
    return () => {
      alive.current = false
    }
  }, [])

  return (
    <MobileDirectThread
      conversationID=""
      sendDirectMessage={async (input) => {
        const result = await sendFirstDirectTextMessage({
          targetIdentityId: user.identityId,
          ...input,
        })
        // 首条消息持久化后，使已有会话查找和历史缓存失效。
        void invalidate(resourceKeys.directConversation(user.identityId))
        void invalidate(
          resourceKeys.conversationMessages(result.conversation.id),
        )
        if (alive.current) {
          void navigate(`/inbox/direct/${result.conversation.id}`, {
            replace: true,
            state: {
              ...location.state,
              conversation: result.conversation,
              memberUserID: user.id,
            },
          })
        }
        return result.message
      }}
    />
  )
}
