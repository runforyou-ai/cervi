/** 通讯录列表的加载、错误状态容器和分页控件。 */
import { useLayoutEffect, useRef, type ReactNode } from "react"
import { useLocation } from "react-router"
import { useTranslation } from "react-i18next"

import { PageControls } from "@/components/page-controls"
import type { PageInfo } from "@/api"
import { PageContent } from "@/components/page-content"
import { ResourceContent } from "@/components/resource-content"

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
  const { t } = useTranslation("contacts")
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
      <ResourceContent
        loading={loading}
        error={error}
        errorMessage={t("list.loadError")}
        onRetry={onRetry}
      >
        <div className="overflow-hidden rounded-lg border bg-card">
          {children}
          <PageControls page={page} onPageChange={onPageChange} />
        </div>
      </ResourceContent>
    </PageContent>
  )
}
