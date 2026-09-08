/** 展示资源的首次加载、失败重试和就绪内容。 */
import type { ReactNode } from "react"
import { useTranslation } from "react-i18next"

import { LoadingIndicator } from "@/components/loading-indicator"
import { Button } from "@/components/ui/button"

/** 为管理页面保留一致的加载与重试区域。 */
export function ResourceContent({
  loading,
  error,
  errorMessage,
  onRetry,
  children,
}: {
  loading: boolean
  error: boolean
  errorMessage: string
  onRetry: () => void
  children: ReactNode
}) {
  const { t } = useTranslation("common")
  if (loading)
    return (
      <LoadingIndicator className="min-h-48 justify-center rounded-lg border">
        {t("status.loading")}
      </LoadingIndicator>
    )
  if (error)
    return (
      <div className="flex min-h-48 flex-col items-center justify-center rounded-lg border text-center">
        <p className="text-sm text-muted-foreground">{errorMessage}</p>
        <Button type="button" className="mt-4" variant="outline" onClick={onRetry}>
          {t("actions.retry")}
        </Button>
      </div>
    )
  return children
}
