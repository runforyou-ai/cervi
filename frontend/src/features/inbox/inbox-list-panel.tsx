/** 提供尺寸稳定的列表工具区、滚动容器和局部加载重试入口。 */
import { useEffect, type ReactNode } from "react"
import { useTranslation } from "react-i18next"
import { Button } from "@/components/ui/button"
import { ScrollArea } from "@/components/ui/scroll-area"
import type { InboxList } from "./use-inbox-list"
import type { useInboxListViewport } from "./use-inbox-list-viewport"

/** 在当前滚动边界自动补页，失败后等待用户原位重试。 */
export function InboxListPanel({ list, viewport, detailError, retryDetail, children }: {
  list: InboxList
  viewport: ReturnType<typeof useInboxListViewport>
  detailError: boolean
  retryDetail: () => void
  children: ReactNode
}) {
  const { t } = useTranslation(["inbox", "common"])
  const { root, interaction } = viewport
  const busy = list.operation !== null
  useEffect(() => {
    const container = root.current?.querySelector<HTMLElement>("[data-slot=scroll-area-viewport]")
    if (!container) return
    // 首屏不足一屏时补齐；已在深处的窗口不主动向前吞入新会话。
    if (!busy && !list.error && list.hasAfter && container.clientHeight > 0 && container.scrollHeight <= container.clientHeight + 120) void list.request("after")
  }, [busy, list.error, list.hasAfter, list.request, list.revision, root])

  return (
    <div ref={root} className="flex min-h-0 flex-1 flex-col" data-slot="inbox-list-panel">
      <div className="flex h-10 shrink-0 items-center gap-2 border-b px-3" data-slot="inbox-list-tools">
        <span role="status" className="min-w-0 flex-1 truncate text-xs text-muted-foreground">
          {list.error ? t("inboxRefreshError") : detailError ? t("conversationLoadError") : busy ? t("common:status.loading") : list.pendingChanges ? t("listPendingChanges") : ""}
        </span>
        <Button variant="outline" size="sm" onClick={() => { void list.retry(); if (detailError) retryDetail() }}>
          {t(list.error || detailError ? "common:actions.retry" : "common:actions.refresh")}
        </Button>
        <Button variant="outline" size="sm" onClick={() => void list.request("latest")}>
          {t("listViewLatest")}
        </Button>
      </div>
      <ScrollArea
        className="min-h-0 min-w-0 flex-1 [&>[data-slot=scroll-area-viewport]]:[overflow-anchor:none] [&>[data-slot=scroll-area-viewport]>div]:block"
        onPointerDownCapture={() => { interaction.current.pointer = true }}
        onPointerUpCapture={() => { interaction.current.pointer = false }}
        onPointerCancelCapture={() => { interaction.current.pointer = false }}
        onScrollCapture={(event) => {
          const container = event.target as HTMLElement
          interaction.current.scrollingUntil = performance.now() + 180
          if (list.error || !container.clientHeight) return
          if (container.scrollTop <= 120 && list.hasBefore) void list.request("before")
          else if (container.scrollHeight - container.scrollTop - container.clientHeight <= 120 && list.hasAfter) void list.request("after")
        }}
      >
        <div className="grid min-w-0 pb-1.5">
          {children}
          {!list.conversations.length && (list.hasBefore || list.hasAfter) && list.revision > 0 ? (
            <p className="px-3 py-6 text-center text-sm text-muted-foreground">{t("listWindowEmpty")}</p>
          ) : null}
          <div className="flex h-10 items-center justify-center gap-2 text-xs text-muted-foreground" data-slot="inbox-list-footer">
            {!list.conversations.length && list.hasBefore ? (
              <Button variant="outline" size="sm" onClick={() => void list.request("before")}>{t("common:pagination.previous")}</Button>
            ) : null}
            {list.error === "before" || list.error === "after" ? (
              <Button variant="outline" size="sm" onClick={() => void list.retry()}>{t("common:actions.retry")}</Button>
            ) : list.status === "loadingMore" ? t("common:status.loading") : list.hasAfter ? (
              <Button variant="outline" size="sm" onClick={() => void list.request("after")}>{t("common:actions.loadMore")}</Button>
            ) : null}
          </div>
        </div>
      </ScrollArea>
    </div>
  )
}
