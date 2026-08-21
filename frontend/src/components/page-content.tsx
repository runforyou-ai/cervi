/** 主内容区的标准滚动容器。 */
import type { ComponentProps } from "react"

import { cn } from "@/lib/utils"

/** 使用统一留白承载页面主体内容。 */
export function PageContent({
  className,
  ...props
}: ComponentProps<"div">) {
  return (
    <div
      data-slot="page-content"
      className={cn(
        "min-h-0 flex-1 overflow-auto p-4 sm:p-6",
        className,
      )}
      {...props}
    />
  )
}
