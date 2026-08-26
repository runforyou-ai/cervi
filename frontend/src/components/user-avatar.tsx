/** 企业成员头像展示。 */
import { useEffect, useState } from "react"

import type { CurrentUser } from "@/api"
import { cn } from "@/lib/utils"

/** 展示用户头像，图片不可用时回退到姓名首字。 */
export function UserAvatar({
  user,
  className,
}: {
  user: CurrentUser
  className?: string
}) {
  const [failed, setFailed] = useState(false)

  useEffect(() => setFailed(false), [user.avatarUrl])

  return (
    <span
      className={cn(
        "flex shrink-0 items-center justify-center overflow-hidden bg-sidebar-primary font-semibold text-sidebar-primary-foreground",
        className,
      )}
    >
      {user.avatarUrl && !failed ? (
        <img
          className="size-full object-cover"
          src={user.avatarUrl}
          alt={user.displayName}
          onError={() => setFailed(true)}
        />
      ) : (
        user.displayName.slice(0, 1).toUpperCase()
      )}
    </span>
  )
}
