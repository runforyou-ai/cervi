/** 消息列表路由。 */
import { useEffect } from "react"
import { LoaderCircleIcon, RefreshCwIcon } from "lucide-react"
import { useTranslation } from "react-i18next"

import { loadInbox } from "@/api"
import { Button } from "@/components/ui/button"
import { InboxPage } from "@/features/inbox/inbox-page"
import { useWorkspace } from "@/contexts/workspace-context"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"

/** 加载并显示消息页。 */
export function InboxRoute() {
  const { t } = useTranslation("workspace")
  const { applyUnreadSnapshot, beginUnreadSnapshot } = useWorkspace()
  const { data, loading, refreshing, error, refresh } = useResource(
    resourceKeys.inbox(),
    () => loadInbox(),
  )
  const showLoading = loading || (Boolean(error) && refreshing)

  /** 数据就绪或更新后同步未读快照；命中缓存的重新挂载同样生效。 */
  useEffect(() => {
    if (!data) return
    const unreadRevision = beginUnreadSnapshot()
    /* 未读事实属于后续阶段，当前快照恒为零。 */
    applyUnreadSnapshot(0, unreadRevision)
    console.info("消息已加载", {
      conversation_count: data.conversations.length,
    })
  }, [applyUnreadSnapshot, beginUnreadSnapshot, data])

  if (showLoading) {
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
            {t("inboxLoadError")}
          </p>
          <Button
            className="mt-4"
            variant="outline"
            onClick={() => void refresh()}
          >
            <RefreshCwIcon />
            {t("retry")}
          </Button>
        </div>
      </div>
    )
  }

  return <InboxPage conversations={data.conversations} />
}
