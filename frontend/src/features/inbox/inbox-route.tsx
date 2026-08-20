/** 消息列表路由。 */
import { useCallback, useEffect, useState } from "react"
import { LoaderCircleIcon, RefreshCwIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"

import { loadInbox, recoverSession, type InboxData } from "@/api"
import { Button } from "@/components/ui/button"
import { InboxPage } from "@/features/inbox/inbox-page"

/** 校验登录后显示消息页。 */
export function InboxRoute() {
  const { t } = useTranslation("workspace")
  const navigate = useNavigate()
  const [data, setData] = useState<InboxData | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState("")

  /** 加载消息列表。 */
  const fetchInbox = useCallback(async () => {
    setLoading(true)
    setError("")
    try {
      const inbox = await loadInbox()
      setData(inbox)
      console.info("消息已加载", {
        conversation_count: inbox.conversations.length,
      })
    } catch (requestError) {
      if (recoverSession(requestError, navigate)) {
        return
      }
      console.warn("消息加载失败", requestError)
      setError(t("inboxLoadError"))
    } finally {
      setLoading(false)
    }
  }, [navigate, t])

  useEffect(() => {
    void fetchInbox()
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
