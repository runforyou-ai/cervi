/** 列表行内的文字操作按钮。 */
import type { ComponentProps } from "react"

import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"

/** 按语义配色展示一行内的文字操作。 */
export function ListActionButton({
  tone = "default",
  className,
  ...props
}: ComponentProps<typeof Button> & {
  tone?: "default" | "destructive" | "success"
}) {
  return (
    <Button
      variant="ghost"
      size="xs"
      className={cn(
        "rounded-xl px-3",
        tone === "default" &&
          "bg-primary/10 text-primary hover:bg-primary/20 hover:text-primary",
        tone === "destructive" &&
          "bg-destructive/10 text-destructive hover:bg-destructive/20 hover:text-destructive",
        tone === "success" &&
          "bg-success/10 text-success hover:bg-success/20 hover:text-success",
        className,
      )}
      {...props}
    />
  )
}
