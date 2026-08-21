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
      className="flex min-h-14 shrink-0 flex-wrap items-center gap-3 border-b px-4 py-3 select-none sm:px-6 md:h-14 md:flex-nowrap md:py-0"
    >
      {beforeTitle}
      <div className="mr-auto min-w-0 flex-1">
        <h2
          data-slot="page-header-title"
          className="w-fit max-w-full truncate text-base leading-6 font-semibold tracking-tight"
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
