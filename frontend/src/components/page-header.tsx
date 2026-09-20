/** 主内容区的标准页头。 */
import type { ReactNode } from "react"

import { SelectableText } from "@/components/selectable-text"

/** 显示统一的标题、前置内容和操作区。 */
export function PageHeader({
  title,
  beforeTitle,
  children,
}: {
  title: ReactNode
  beforeTitle?: ReactNode
  children?: ReactNode
}) {
  return (
    <header
      data-slot="page-header"
      className="cervi-page-gutter flex min-h-12 shrink-0 flex-wrap items-center gap-2.5 py-2.5 select-none md:h-12 md:flex-nowrap md:py-0"
    >
      {beforeTitle}
      <div className="mr-auto min-w-0 flex-1">
        <h2
          data-slot="page-header-title"
          className="w-fit max-w-full truncate text-xl font-semibold tracking-tight"
        >
          <SelectableText>{title}</SelectableText>
        </h2>
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
