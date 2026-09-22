/** 列表页的滚动容器、加载状态、表格容器和分页。 */
import type { ComponentProps, ReactNode } from "react"
import { useLocation } from "react-router"

import type { PageInfo } from "@/api"
import { PageContent } from "@/components/page-content"
import { PageControls } from "@/components/page-controls"
import {
  ResourceContent,
  resourceStatus,
  type ResourceState,
} from "@/components/resource-content"
import { useListScrollRestore } from "@/hooks/use-list-scroll-restore"
import { cn } from "@/lib/utils"

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
  resources,
  errorMessage,
  page,
  disabled,
  onPageChange,
  frameClassName,
  children,
}: {
  resources: ResourceState | readonly ResourceState[]
  errorMessage: string
  page?: PageInfo
  disabled?: boolean
  onPageChange?: (page: number) => void
  frameClassName?: string
  children: ReactNode
}) {
  const location = useLocation()
  const scroll = useListScrollRestore(
    location.pathname + location.search,
    resourceStatus(resources).status === "ready",
  )

  return (
    <PageContent ref={scroll.ref} onScroll={scroll.onScroll}>
      <ResourceContent resources={resources} errorMessage={errorMessage}>
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
