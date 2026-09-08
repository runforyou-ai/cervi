/** 列表共用的总数、页码和翻页操作。 */
import type { ReactNode } from "react"
import { useTranslation } from "react-i18next"

import type { PageInfo } from "@/api"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"

/** 根据服务端分页信息展示翻页边界并禁用加载中的操作。 */
export function PageControls({
  page,
  disabled = false,
  totalLabel,
  className,
  onPageChange,
}: {
  page: PageInfo
  disabled?: boolean
  totalLabel?: ReactNode
  className?: string
  onPageChange: (page: number) => void
}) {
  const { t } = useTranslation("common")
  const totalPages = Math.max(1, Math.ceil(page.total / page.size))
  return (
    <div
      className={cn(
        "flex items-center justify-between gap-3 border-t px-4 py-3 text-sm text-muted-foreground",
        className,
      )}
    >
      <span>{totalLabel ?? t("pagination.total", { count: page.total })}</span>
      <div className="flex items-center gap-2">
        <Button
          type="button"
          variant="outline"
          size="sm"
          disabled={disabled || page.number <= 1}
          onClick={() => onPageChange(page.number - 1)}
        >
          {t("pagination.previous")}
        </Button>
        <span>
          {t("pagination.page", { current: page.number, total: totalPages })}
        </span>
        <Button
          type="button"
          variant="outline"
          size="sm"
          disabled={disabled || page.number >= totalPages}
          onClick={() => onPageChange(page.number + 1)}
        >
          {t("pagination.next")}
        </Button>
      </div>
    </div>
  )
}
