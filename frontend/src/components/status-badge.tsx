/** 通用状态徽标。 */
import type { ComponentProps } from "react"

import { cn } from "@/lib/utils"

/** 按语义显示成功或中性状态。 */
export function StatusBadge({
  variant,
  className,
  children,
  ...props
}: ComponentProps<"span"> & { variant: "success" | "muted" }) {
  return (
    <span
      className={cn(
        "inline-flex items-center font-medium",
        variant === "success"
          ? "gap-1.5 rounded-full bg-success/15 px-2 py-0.5 text-xs text-success"
          : "rounded-sm bg-muted px-1.5 py-0.5 text-[10px] text-muted-foreground",
        className,
      )}
      {...props}
    >
      {variant === "success" ? (
        <span aria-hidden="true" className="size-1.5 rounded-full bg-success" />
      ) : null}
      {children}
    </span>
  )
}
