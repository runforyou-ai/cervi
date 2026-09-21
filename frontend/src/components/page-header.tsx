/** 主内容区的标准页头。 */
import type { ReactNode } from "react"

import { SelectableText } from "@/components/selectable-text"

/** 显示统一的标题、说明、前置内容和操作区。 */
export function PageHeader({
  title,
  description,
  beforeTitle,
  children,
}: {
  title: ReactNode
  description?: ReactNode
  beforeTitle?: ReactNode
  children?: ReactNode
}) {
  return (
    <header
      data-slot="page-header"
      className="cervi-page-gutter flex min-h-16 shrink-0 flex-wrap items-center gap-2.5 py-3.5 select-none md:flex-nowrap"
    >
      {beforeTitle}
      <div className="mr-auto min-w-0 flex-1">
        <h2
          data-slot="page-header-title"
          className="w-fit max-w-full truncate text-2xl font-semibold tracking-tight"
        >
          <SelectableText>{title}</SelectableText>
        </h2>
        {description ? (
          <p
            data-slot="page-header-description"
            className="mt-1 truncate text-sm text-muted-foreground"
          >
            {description}
          </p>
        ) : null}
      </div>
      {children ? (
        <div
          data-slot="page-header-actions"
          className="flex shrink-0 items-center gap-2"
        >
          {children}
        </div>
      ) : null}
    </header>
  )
}
