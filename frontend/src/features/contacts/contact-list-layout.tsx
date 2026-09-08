/** 通讯录列表的加载、错误状态容器和分页控件。 */
import type { ReactNode } from "react"
import { useTranslation } from "react-i18next"

import { PageControls } from "@/components/page-controls"
import type { PageInfo } from "@/api"
import { LoadingIndicator } from "@/components/loading-indicator"
import { PageContent } from "@/components/page-content"
import { Button } from "@/components/ui/button"

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
  return (
    <PageContent>
      {loading ? (
        <LoadingIndicator className="min-h-48 justify-center rounded-lg border">
          {t("common:status.loading")}
        </LoadingIndicator>
      ) : error ? (
        <div className="flex min-h-48 flex-col items-center justify-center rounded-lg border text-center">
          <p className="text-sm text-muted-foreground">
            {t("list.loadError")}
          </p>
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
