/** 导航与页签共用的数量徽标。 */
import { cn } from "@/lib/utils"

/** 红底白字显示需要关注的数量，超过 99 显示 99+；muted 时改用弱化配色。 */
export function CountBadge({
  count,
  label,
  muted = false,
  className,
}: {
  count: number
  label?: string
  muted?: boolean
  className?: string
}) {
  return (
    <span
      aria-label={label}
      className={cn(
        "flex h-4 min-w-4 shrink-0 items-center justify-center rounded-full px-1 text-[10px] leading-none font-semibold tabular-nums",
        muted ? "bg-muted text-muted-foreground" : "bg-destructive text-destructive-foreground",
        className,
      )}
    >
      {count > 99 ? "99+" : count}
    </span>
  )
}
