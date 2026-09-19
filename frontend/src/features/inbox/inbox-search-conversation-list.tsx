/** 会话名称搜索的「查看全部」分页列表：复用收件箱列表控制器分页读取，并随同步失效重读。 */
import { useEffect, type ReactNode } from "react"
import { useTranslation } from "react-i18next"

import type { Identity, InboxConversation, InboxQuery } from "@/api"
import { InboxListPanel } from "./inbox-list-panel"
import { useInboxList } from "./use-inbox-list"
import { useInboxListViewport } from "./use-inbox-list-viewport"

/** 按名称搜索查询分页展示全部命中会话，行元素需带 data-inbox-id 供滚动补偿定位。 */
export function InboxSearchConversationList({
  identity,
  query,
  mobile = false,
  onConversationsChange,
  children,
}: {
  identity: Identity
  query: InboxQuery
  mobile?: boolean
  onConversationsChange?: (conversations: InboxConversation[]) => void
  children: (conversations: InboxConversation[]) => ReactNode
}) {
  const { t } = useTranslation("inbox")
  const viewport = useInboxListViewport()
  const list = useInboxList(query, viewport, { identity, active: true })
  const { conversations } = list
  const ids = conversations.map((conversation) => conversation.id).join(",")

  useEffect(() => {
    // 行集合变化时同步给键盘选择，内容更新不改变选择位置。
    onConversationsChange?.(conversations)
  }, [ids, onConversationsChange])

  // 读取完成且前后都没有更多命中时才展示无结果。
  const empty = list.revision > 0 && !list.error && !list.ids.length && !list.hasBefore && !list.hasAfter
  return (
    <InboxListPanel list={list} viewport={viewport} mobile={mobile}>
      {empty ? (
        <p className="px-6 py-10 text-center text-[13px] text-muted-foreground">{t("searchNoResults", { query: query.search })}</p>
      ) : (
        children(conversations)
      )}
    </InboxListPanel>
  )
}
