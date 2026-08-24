/** 消息列表路由。 */
import { useCallback, useEffect, useRef, useState } from "react"
import { LoaderCircleIcon, RefreshCwIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"

import { loadInbox, type InboxData } from "@/api"
import { recoverSession } from "@/lib/session-navigation"
import { Button } from "@/components/ui/button"
import { InboxPage } from "@/features/inbox/inbox-page"
import { useWorkspace } from "@/features/workspace/workspace-context"

/** 校验登录后显示消息页。 */
export function InboxRoute() {
  const { t } = useTranslation("workspace")
  const navigate = useNavigate()
  const { applyUnreadSnapshot, beginUnreadSnapshot } = useWorkspace()
  const [data, setData] = useState<InboxData | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState("")
  const mountedRef = useRef(false)
  const requestIdRef = useRef(0)

  /** 加载消息列表。 */
  const fetchInbox = useCallback(async () => {
    const requestId = ++requestIdRef.current
    const unreadRevision = beginUnreadSnapshot()
    setLoading(true)
    setError("")
    try {
      const inbox = await loadInbox()
      if (!mountedRef.current || requestId !== requestIdRef.current) {
        return
      }
      setData(inbox)
      const unreadCount = inbox.conversations.reduce(
        (total, conversation) => total + (conversation.unread ?? 0),
        0,
      )
      applyUnreadSnapshot(unreadCount, unreadRevision)
      console.info("消息已加载", {
        conversation_count: inbox.conversations.length,
        unread_count: unreadCount,
      })
    } catch (requestError) {
      if (!mountedRef.current || requestId !== requestIdRef.current) {
        return
      }
      if (recoverSession(requestError, navigate)) {
        return
      }
      console.warn("消息加载失败", requestError)
      setError(t("inboxLoadError"))
    } finally {
      if (mountedRef.current && requestId === requestIdRef.current) {
        setLoading(false)
      }
    }
  }, [applyUnreadSnapshot, beginUnreadSnapshot, navigate, t])

  useEffect(() => {
    mountedRef.current = true
    void fetchInbox()
    return () => {
      mountedRef.current = false
      requestIdRef.current += 1
    }
  }, [fetchInbox])

  if (loading) {
    return (
      <div className="flex flex-1 items-center justify-center gap-2 text-sm text-muted-foreground">
        <LoaderCircleIcon className="size-4 animate-spin" />
        {t("loading")}
      </div>
    )
  }

  if (!data) {
    return (
      <div className="flex flex-1 items-center justify-center p-6 text-center">
        <div>
          <p className="text-sm text-muted-foreground">
            {error}
          </p>
          <Button className="mt-4" variant="outline" onClick={fetchInbox}>
            <RefreshCwIcon />
            {t("retry")}
          </Button>
        </div>
      </div>
    )
  }

  return <InboxPage conversations={data.conversations} />
}
