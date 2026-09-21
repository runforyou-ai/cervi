/** 列表页的滚动容器、加载状态、表格容器和分页。 */
import { useLayoutEffect, useRef, type ComponentProps, type ReactNode } from "react"
import { useLocation } from "react-router"

import type { PageInfo } from "@/api"
import { PageContent } from "@/components/page-content"
import { PageControls } from "@/components/page-controls"
import { ResourceContent } from "@/components/resource-content"
import { cn } from "@/lib/utils"

const listScrollPositions = new Map<string, number>()

/** 列表表格的容器，给出分页信息时在底部渲染翻页。 */
export function ResourceListFrame({
  page,
  disabled,
  totalLabel,
  onPageChange,
  className,
  children,
  ...props
}: ComponentProps<"div"> & {
  page?: PageInfo
  disabled?: boolean
  totalLabel?: ReactNode
  onPageChange?: (page: number) => void
}) {
  return (
    <div
      data-slot="resource-list-frame"
      className={cn("flex flex-col", className)}
      {...props}
    >
      <div className="overflow-hidden rounded-lg border border-border/55">{children}</div>
      {page && onPageChange ? (
        <PageControls
          page={page}
          disabled={disabled}
          totalLabel={totalLabel}
          onPageChange={onPageChange}
        />
      ) : null}
    </div>
  )
}

/** 列表页主体：恢复滚动位置，按加载状态渲染表格容器与分页。 */
export function ResourceListLayout({
  loading,
  error,
  errorMessage,
  onRetry,
  page,
  disabled,
  onPageChange,
  frameClassName,
  children,
}: {
  loading: boolean
  error: boolean
  errorMessage: string
  onRetry: () => void
  page?: PageInfo
  disabled?: boolean
  onPageChange?: (page: number) => void
  frameClassName?: string
  children: ReactNode
}) {
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
        errorMessage={errorMessage}
        onRetry={onRetry}
      >
        <ResourceListFrame
          page={page}
          disabled={disabled}
          onPageChange={onPageChange}
          className={frameClassName}
        >
          {children}
        </ResourceListFrame>
      </ResourceContent>
    </PageContent>
  )
}
