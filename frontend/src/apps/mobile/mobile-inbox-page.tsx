/** 移动端收件箱：待处理与全部服务会话列表、筛选和页签滑动切换。 */
import { SearchIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"

import { InboxScope } from "@/api"
import {
  MobileConversationListPage,
  useMobileListOptions,
} from "@/apps/mobile/mobile-conversation-list"
import {
  MobileInboxFilter,
  MobileInboxScopes,
  useMobileInboxQuery,
} from "@/apps/mobile/mobile-inbox-navigation"
import { mobileSearchPath } from "@/apps/mobile/mobile-navigation"
import { useMobileWorkspace } from "@/apps/mobile/mobile-workspace-layout"
import { Button } from "@/components/ui/button"
import { useInboxAttention } from "@/features/inbox/inbox-attention"
import { inboxTabs } from "@/features/inbox/inbox-query"
import {
  useInboxList,
  usePartitionedInboxList,
  type InboxList,
  type InboxListViewport,
  type PartitionedInboxList,
} from "@/features/inbox/use-inbox-list"
import { useInboxListViewport } from "@/features/inbox/use-inbox-list-viewport"

/** 按当前页签和筛选挂载独立列表窗口，离开时保存原邻域。 */
export function MobileInboxPage() {
  const navigation = useMobileInboxQuery()
  const key = JSON.stringify(navigation.query)
  return navigation.query.scope === InboxScope.InboxScopePending
    ? <MobilePendingInbox key={key} {...navigation} />
    : <MobileAllInbox key={key} {...navigation} />
}

/** 待处理页签按等待起点读取一条列表。 */
function MobilePendingInbox(navigation: ReturnType<typeof useMobileInboxQuery>) {
  const viewport = useInboxListViewport()
  const list = useInboxList(navigation.query, viewport, useMobileListOptions())
  return <MobileInboxList {...navigation} list={list} viewport={viewport} />
}

/** 全部页签按置顶区在前、最近活动在后读取列表。 */
function MobileAllInbox(navigation: ReturnType<typeof useMobileInboxQuery>) {
  const viewport = useInboxListViewport()
  const list = usePartitionedInboxList(navigation.query, viewport, useMobileListOptions())
  return <MobileInboxList {...navigation} list={list} viewport={viewport} />
}

/** 渲染收件箱标题、页签、筛选和服务会话列表，左右滑动在待处理与全部之间切换。 */
function MobileInboxList({
  query,
  changeQuery,
  list,
  viewport,
}: ReturnType<typeof useMobileInboxQuery> & {
  list: InboxList | PartitionedInboxList
  viewport: InboxListViewport
}) {
  const { t } = useTranslation(["mobile", "inbox", "common"])
  const navigate = useNavigate()
  const { identity } = useMobileWorkspace()
  const attention = useInboxAttention(identity)
  const pending = query.scope === InboxScope.InboxScopePending
  return (
    <MobileConversationListPage
      title={t("inbox.title")}
      actions={
        <Button
          variant="ghost"
          size="icon-lg"
          className="shrink-0"
          aria-label={t("common:actions.search")}
          onClick={() => navigate(mobileSearchPath(), { state: { mobileBack: true } })}
        >
          <SearchIcon />
        </Button>
      }
      toolbar={
        <>
          <MobileInboxScopes
            scope={query.scope}
            pendingCount={attention.data?.pending ?? 0}
            onChange={changeQuery}
          />
          <MobileInboxFilter query={query} onChange={changeQuery} onOpenChange={viewport.setMenu} />
        </>
      }
      list={list}
      viewport={viewport}
      showAssignee={!pending}
      showAudience
      emptyTitle={pending ? t("inbox:pendingEmptyTitle") : t("inbox:emptyTitle")}
      emptyDescription={pending ? t("inbox:pendingEmptyDescription") : t("inbox:emptyDescription")}
      onSwipe={(direction) => {
        const current = inboxTabs.findIndex((item) => item.value === query.scope)
        const next = inboxTabs[current + direction]
        if (!next) return false
        changeQuery({ scope: next.value })
        return true
      }}
    />
  )
}
