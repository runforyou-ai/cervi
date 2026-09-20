/** 列表共用的总数、页码和翻页操作。 */
import type { ReactNode } from "react"
import { ChevronLeftIcon, ChevronRightIcon } from "lucide-react"
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
        "flex items-center justify-between gap-3 px-3 pt-3 text-xs text-muted-foreground",
        className,
      )}
    >
      <span>{totalLabel ?? t("pagination.total", { count: page.total })}</span>
      {totalPages > 1 ? (
        <div className="flex items-center gap-1">
          <Button
            type="button"
            variant="ghost"
            size="icon-sm"
            aria-label={t("pagination.previous")}
            disabled={disabled || page.number <= 1}
            onClick={() => onPageChange(page.number - 1)}
          >
            <ChevronLeftIcon />
          </Button>
          <span className="px-1">
            {t("pagination.page", { current: page.number, total: totalPages })}
          </span>
          <Button
            type="button"
            variant="ghost"
            size="icon-sm"
            aria-label={t("pagination.next")}
            disabled={disabled || page.number >= totalPages}
            onClick={() => onPageChange(page.number + 1)}
          >
            <ChevronRightIcon />
          </Button>
        </div>
      ) : null}
    </div>
  )
}
