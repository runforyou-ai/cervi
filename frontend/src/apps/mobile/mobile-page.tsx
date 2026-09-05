/** 移动端统一标题、列表滚动区域和页面状态。 */
import { useEffect, useLayoutEffect, useRef, type ReactNode } from "react"
import { ArrowLeftIcon } from "lucide-react"
import { useTranslation } from "react-i18next"

import {
  useMobileBack,
  useMobileNavigation,
} from "@/apps/mobile/mobile-navigation"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"

/** 渲染一级页或详情页标题，固定操作区的位置。 */
export function MobilePageHeader({
  title,
  backTo,
  actions,
}: {
  title: ReactNode
  backTo?: string
  actions?: ReactNode
}) {
  const { t } = useTranslation("common")
  const back = useMobileBack(backTo ?? "/inbox")
  useEffect(() => {
    if (!backTo) return
    // 浮层处理后，详情页让系统返回与标题返回走同一条路径。
    const handleBack = (event: Event) => {
      if (event.defaultPrevented) return
      event.preventDefault()
      back()
    }
    window.addEventListener("cervi:back", handleBack)
    return () => window.removeEventListener("cervi:back", handleBack)
  }, [back, backTo])
  return (
    <header className="flex h-14 shrink-0 items-center gap-2 border-b px-4">
      {backTo ? (
        <Button
          className="-ml-2"
          variant="ghost"
          size="icon-lg"
          aria-label={t("actions.back")}
          onClick={back}
        >
          <ArrowLeftIcon />
        </Button>
      ) : null}
      <h1 className="min-w-0 flex-1 truncate text-lg font-semibold tracking-tight">
        {title}
      </h1>
      {actions}
    </header>
  )
}

/** 按页面和筛选条件保存滚动位置，数据就绪后恢复。 */
export function MobileScrollArea({
  storageKey,
  ready = true,
  children,
  className,
}: {
  storageKey: string
  ready?: boolean
  children: ReactNode
  className?: string
}) {
  const { scrollPositions } = useMobileNavigation()
  const element = useRef<HTMLDivElement>(null)
  useLayoutEffect(() => {
    if (ready && element.current) {
      element.current.scrollTop = scrollPositions.get(storageKey) ?? 0
    }
  }, [ready, scrollPositions, storageKey])
  return (
    <div
      key={storageKey}
      ref={element}
      className={cn(
        "min-h-0 flex-1 overflow-y-auto overscroll-contain",
        className,
      )}
      onScroll={(event) => {
        if (ready)
          scrollPositions.set(storageKey, event.currentTarget.scrollTop)
      }}
    >
      {children}
    </div>
  )
}

/** 显示空白、未开放或加载失败状态，并按需提供重试。 */
export function MobilePageState({
  title,
  description,
  onRetry,
}: {
  title: string
  description?: string
  onRetry?: () => void
}) {
  const { t } = useTranslation("mobile")
  return (
    <div
      className="flex min-h-64 flex-1 flex-col items-center justify-center px-6 py-12 text-center"
      role="status"
    >
      <p className="text-base font-medium">{title}</p>
      {description ? (
        <p className="mt-2 max-w-xs text-sm leading-6 text-muted-foreground">
          {description}
        </p>
      ) : null}
      {onRetry ? (
        <Button className="mt-4 min-h-11" variant="outline" onClick={onRetry}>
          {t("retry")}
        </Button>
      ) : null}
    </div>
  )
}
