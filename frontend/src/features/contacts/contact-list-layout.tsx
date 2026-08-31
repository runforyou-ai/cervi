/** 通讯录列表的加载、错误状态容器和分页控件。 */
import type { ReactNode } from "react"
import {
  ChevronLeftIcon,
  ChevronRightIcon,
} from "lucide-react"
import { useTranslation } from "react-i18next"

import type { PageInfo } from "@/api"
import { LoadingIndicator } from "@/components/loading-indicator"
import { PageContent } from "@/components/page-content"
import { Button } from "@/components/ui/button"

/** 联系人列表分页。 */
function PageControls({
  page,
  onPageChange,
}: {
  page: PageInfo
  onPageChange: (page: number) => void
}) {
  const { t } = useTranslation("contacts")
  const totalPages = Math.max(1, Math.ceil(page.total / page.size))
  return (
    <div className="flex items-center justify-between border-t px-4 py-3 text-sm text-muted-foreground">
      <span>{t("pagination.total", { count: page.total })}</span>
      <div className="flex items-center gap-2">
        <Button
          variant="outline"
          size="sm"
          disabled={page.number <= 1}
          onClick={() => onPageChange(page.number - 1)}
        >
          <ChevronLeftIcon />
          {t("pagination.previous")}
        </Button>
        <span>
          {t("pagination.page", { current: page.number, total: totalPages })}
        </span>
        <Button
          variant="outline"
          size="sm"
          disabled={page.number >= totalPages}
          onClick={() => onPageChange(page.number + 1)}
        >
          {t("pagination.next")}
          <ChevronRightIcon />
        </Button>
      </div>
    </div>
  )
}

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
  return (
    <PageContent>
      {loading ? (
        <LoadingIndicator className="min-h-48 justify-center rounded-lg border">
          {t("loading")}
        </LoadingIndicator>
      ) : error ? (
        <div className="flex min-h-48 flex-col items-center justify-center rounded-lg border text-center">
          <p className="text-sm text-muted-foreground">
            {t("list.loadError")}
          </p>
          <Button className="mt-4" variant="outline" onClick={onRetry}>
            {t("retry")}
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
