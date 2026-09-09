/** 通讯录列表的加载、错误状态容器和分页控件。 */
import { useLayoutEffect, useRef, type ReactNode } from "react"
import { useLocation } from "react-router"
import { useTranslation } from "react-i18next"

import { PageControls } from "@/components/page-controls"
import type { PageInfo } from "@/api"
import { LoadingIndicator } from "@/components/loading-indicator"
import { PageContent } from "@/components/page-content"
import { Button } from "@/components/ui/button"

const listScrollPositions = new Map<string, number>()

/** 按加载状态包裹列表表格并渲染分页。 */
export function ContactListLayout({
  loading,
  error,
  onRetry,
  page,
  onPageChange,
  children,
}: {
  loading: boolean
  error: boolean
  onRetry: () => void
  page: PageInfo
  onPageChange: (page: number) => void
  children: ReactNode
}) {
  const { t } = useTranslation(["contacts", "common"])
  const location = useLocation()
  const scrollKey = location.pathname + location.search
  const content = useRef<HTMLDivElement>(null)

  // 列表内容就绪后恢复进入表单前的位置。
  useLayoutEffect(() => {
    if (!loading && !error && content.current) {
      content.current.scrollTop = listScrollPositions.get(scrollKey) ?? 0
    }
  }, [loading, error, scrollKey])
  return (
    <PageContent
      ref={content}
      onScroll={(event) => {
        listScrollPositions.set(scrollKey, event.currentTarget.scrollTop)
      }}
    >
      {loading ? (
        <LoadingIndicator className="min-h-48 justify-center rounded-lg border">
          {t("common:status.loading")}
        </LoadingIndicator>
      ) : error ? (
        <div className="flex min-h-48 flex-col items-center justify-center rounded-lg border text-center">
          <p className="text-sm text-muted-foreground">{t("list.loadError")}</p>
          <Button className="mt-4" variant="outline" onClick={onRetry}>
            {t("common:actions.retry")}
          </Button>
        </div>
      ) : (
        <div className="overflow-hidden rounded-lg border bg-card">
          {children}
          <PageControls page={page} onPageChange={onPageChange} />
        </div>
      )}
    </PageContent>
  )
}
