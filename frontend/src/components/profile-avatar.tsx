/** 统一头像图片、默认图案和姓名首字的展示。 */
import { useEffect, useState } from "react"
import { BotIcon, UserRoundIcon, UsersRoundIcon } from "lucide-react"

import { cn } from "@/lib/utils"

/** 展示圆角方形头像，图片不可用时显示默认图案或姓名首字。 */
export function ProfileAvatar({
  imageURL,
  name,
  fallback = "person",
  className,
  title,
}: {
  imageURL?: string
  name?: string | null
  fallback?: "person" | "agent" | "group"
  className?: string
  title?: string
}) {
  const [failedURL, setFailedURL] = useState<string | null>(null)
  useEffect(() => setFailedURL(null), [imageURL])
  const initial = Array.from(name?.trim() ?? "")[0]?.toLocaleUpperCase()

  return (
    <span
      aria-hidden="true"
      title={title}
      className={cn(
        "flex size-10 shrink-0 items-center justify-center overflow-hidden rounded-lg bg-primary/10 text-sm font-medium text-primary",
        className,
      )}
    >
      {imageURL && imageURL !== failedURL ? (
        <img
          key={imageURL}
          src={imageURL}
          alt=""
          className="size-full object-cover"
          draggable={false}
          onError={() => setFailedURL(imageURL)}
        />
      ) : fallback === "agent" ? (
        <BotIcon className="size-[60%]" />
      ) : fallback === "group" ? (
        <UsersRoundIcon className="size-[60%]" />
      ) : initial ? (
        <span className="text-[1.4em] leading-none">{initial}</span>
      ) : (
        <UserRoundIcon className="size-[60%]" />
      )}
    </span>
  )
}
