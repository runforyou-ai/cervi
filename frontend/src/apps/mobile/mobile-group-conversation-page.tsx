/** 移动端已有群聊的资料读取、访问恢复和详情入口。 */
import { useCallback, useEffect, useRef, useState } from "react"
import { BellOffIcon, MoreHorizontalIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import {
  Outlet,
  useLocation,
  useMatch,
  useNavigate,
  useParams,
} from "react-router"
import { toast } from "sonner"

import { getGroupConversation, isNotFoundApiError } from "@/api"
import type { MobileGroupContext } from "@/apps/mobile/mobile-group-context"
import { MobileGroupThread } from "@/apps/mobile/mobile-group-thread"
import { useMobileNavigation } from "@/apps/mobile/mobile-navigation"
import { MobilePageHeader, MobilePageState } from "@/apps/mobile/mobile-page"
import { LoadingIndicator } from "@/components/loading-indicator"
import { Button } from "@/components/ui/button"
import {
  memberChatPollingInterval,
  useMemberChatPollingActive,
} from "@/features/inbox/use-member-chat-polling"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource, useResourceInvalidator } from "@/hooks/use-resource"

/** 按群聊隔离资料查询、发送草稿和访问恢复状态。 */
export function MobileGroupConversationPage() {
  const { conversationID = "" } = useParams()
  return (
    <MobileGroupConversation
      key={conversationID}
      conversationID={conversationID}
    />
  )
}

/** 独立读取群资料，前台同步名称、成员人数及解散状态。 */
function MobileGroupConversation({
  conversationID,
}: {
  conversationID: string
}) {
  const { t } = useTranslation(["mobile", "inbox", "common"])
  const navigate = useNavigate()
  const location = useLocation()
  const navigationState = location.state as {
    mobileBack?: boolean
    groupReturnDepth?: number
  } | null
  // 记录当前群子页到来源列表的历史距离，退出时一次返回来源。
  const returnDepth =
    navigationState?.groupReturnDepth ?? (navigationState?.mobileBack ? 1 : 0)
  const { inboxURL } = useMobileNavigation()
  const invalidate = useResourceInvalidator()
  const leaving = useRef(false)
  const [leavePending, setLeavePending] = useState(false)
  const detailsOpen = !useMatch("/inbox/group/:conversationID")
  const pollingActive = useMemberChatPollingActive({
    requireWindowFocus: false,
  })
  const previousPollingActive = useRef(pollingActive)
  const { data, loading, refreshing, error, refresh } = useResource(
    resourceKeys.groupConversation(conversationID),
    () => getGroupConversation(conversationID),
    {
      staleTime: 0,
      refetchInterval:
        pollingActive && !leavePending ? memberChatPollingInterval : false,
      refetchOnWindowFocus: false,
    },
  )
  const unavailable = !refreshing && isNotFoundApiError(error)

  /** 失去群聊访问权时提示一次，并回到原筛选下的消息列表。 */
  const handleUnavailable = useCallback(() => {
    if (leaving.current) return
    leaving.current = true
    toast.message(t("group.unavailable"))
    void invalidate(resourceKeys.inbox())
    if (returnDepth > 0) void navigate(-returnDepth)
    else void navigate(inboxURL, { replace: true })
  }, [inboxURL, invalidate, navigate, returnDepth, t])

  /** 主动退出后返回来源列表，不再提示访问失效。 */
  function handleLeft() {
    leaving.current = true
    if (returnDepth > 0) void navigate(-returnDepth)
    else void navigate(inboxURL, { replace: true })
  }

  useEffect(() => {
    // 群资料以 not_found 表示不可访问，等待当次校验后再离开。
    if (unavailable && !leavePending) handleUnavailable()
  }, [unavailable, leavePending, handleUnavailable])

  useEffect(() => {
    // 恢复前台立即校验群状态，不等待下一个轮询周期。
    if (pollingActive && !previousPollingActive.current && !leavePending)
      void refresh()
    previousPollingActive.current = pollingActive
  }, [pollingActive, leavePending, refresh])

  return (
    <div className="relative h-full min-h-0">
      <section
        className={`flex h-full min-h-0 flex-col bg-background ${detailsOpen && data ? "absolute inset-0 opacity-0 pointer-events-none" : ""}`}
        inert={Boolean(detailsOpen && data)}
      >
        <MobilePageHeader
          backTo={
            detailsOpen && data
              ? undefined
              : detailsOpen
                ? `/inbox/group/${conversationID}`
                : inboxURL
          }
          title={
            detailsOpen ? (
              t("group.details")
            ) : (
              <span className="flex min-w-0 items-center">
                <span className="truncate">
                  {data?.title ?? t("group.title")}
                </span>
                {data ? (
                  <span className="shrink-0">
                    {t("group.memberCount", {
                      count: data.participants.length,
                    })}
                  </span>
                ) : null}
                {data?.muted ? (
                  <BellOffIcon
                    className="ml-1 size-3.5 shrink-0 text-muted-foreground"
                    aria-label={t("inbox:conversationMuted")}
                  />
                ) : null}
              </span>
            )
          }
          actions={
            <>
              {data && error && !isNotFoundApiError(error) ? (
                <Button
                  variant="ghost"
                  className="min-h-11 text-warning"
                  disabled={refreshing}
                  onClick={() => void refresh()}
                >
                  {t("inbox.refreshFailed")}
                </Button>
              ) : null}
              <Button
                variant="ghost"
                size="icon-lg"
                className="-mr-2"
                aria-label={t("group.details")}
                disabled={!data || detailsOpen}
                onClick={() =>
                  navigate(`/inbox/group/${conversationID}/details`, {
                    replace: returnDepth === 0,
                    state: {
                      mobileBack: returnDepth > 0,
                      groupReturnDepth: returnDepth > 0 ? returnDepth + 1 : 0,
                    },
                  })
                }
              >
                <MoreHorizontalIcon />
              </Button>
            </>
          }
        />
        {data ? (
          <MobileGroupThread
            conversation={data}
            active={!detailsOpen}
            onUnavailable={() => {
              if (!leavePending) handleUnavailable()
            }}
          />
        ) : loading || isNotFoundApiError(error) ? (
          <LoadingIndicator className="min-h-0 flex-1 justify-center">
            {t("common:status.loading")}
          </LoadingIndicator>
        ) : (
          <MobilePageState
            title={t("group.loadError")}
            onRetry={() => void refresh()}
          />
        )}
      </section>
      {data ? (
        <Outlet
          context={
            {
              group: data,
              returnDepth,
              error,
              refreshing,
              refresh,
              onUnavailable: handleUnavailable,
              onLeft: handleLeft,
              onLeavingChange: setLeavePending,
            } satisfies MobileGroupContext
          }
        />
      ) : null}
    </div>
  )
}
