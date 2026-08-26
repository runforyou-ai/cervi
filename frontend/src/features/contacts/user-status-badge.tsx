/** 账号状态徽标。 */
import { UserStatus } from "@/api"
import { cn } from "@/lib/utils"

/** 显示账号状态徽标。 */
export function UserStatusBadge({
  status,
  label,
}: {
  status: UserStatus
  label: string
}) {
  const active = status === UserStatus.UserStatusActive

  return (
    <span
      className={cn(
        "inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium",
        active
          ? "bg-success/15 text-success"
          : "bg-muted text-muted-foreground",
      )}
    >
      {label}
    </span>
  )
}
