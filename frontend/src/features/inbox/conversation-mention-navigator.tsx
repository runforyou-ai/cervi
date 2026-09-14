/** 展示待查看提及导航及返回最新消息入口。 */
import { ChevronDownIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"
import { resolveAppPlatform } from "@/platform/app-platform"
import type { useConversationMentionNavigation } from "./use-conversation-mention-navigation"

/** 保持底部导航位置，区分本轮位置和实时待查看数量；移动端使用带底色的触屏尺寸按钮组。 */
export function ConversationMentionNavigator({
  navigation,
  showLatest,
  newCount,
  busy,
  onLatest,
}: {
  navigation: ReturnType<typeof useConversationMentionNavigation>
  showLatest: boolean
  newCount: number
  busy: boolean
  onLatest: () => void
}) {
  const { t } = useTranslation("inbox")
  const { round } = navigation
  const disabled = busy || navigation.busy
  const mobile = resolveAppPlatform() === "mobile"
  // 移动端按钮高度满足手指点击。
  const buttonClassName = mobile ? "h-11 rounded-full px-4" : undefined
  const latestLabel = newCount > 0
    ? `${t("messagesBackToLatest")} · ${t("messagesNew", { count: newCount })}`
    : t("messagesBackToLatest")
  const hasPrevious = round?.ids.some(
    (id, index) => index < round.index && !round.unavailable.has(id),
  )
  const hasNext = round?.ids.some(
    (id, index) => index > round.index && !round.unavailable.has(id),
  )
  if (!round && !navigation.pendingCount && !showLatest) return null
  return (
    <div
      className="absolute right-4 bottom-3 z-10 flex max-w-[calc(100%-2rem)] flex-wrap items-center justify-end gap-2 p-1"
      aria-label={t("mentionNavigation")}
    >
      {round ? (
        <div
          className={cn(
            "flex flex-wrap items-center justify-end gap-2",
            mobile && "rounded-full border bg-background/95 p-1 pl-2 shadow-sm backdrop-blur",
          )}
        >
          <span className="px-2 text-xs tabular-nums" role="status">
            @ {round.index + 1}/{round.ids.length}
          </span>
          <Button
            size="sm"
            variant="outline"
            className={buttonClassName}
            disabled={disabled || !hasPrevious}
            onClick={() => void navigation.visit(round.index - 1, round, -1)}
          >
            {t("mentionPrevious")}
          </Button>
          {navigation.needsResume ? (
            <Button
              size="sm"
              variant="outline"
              className={buttonClassName}
              disabled={disabled}
              onClick={() => void navigation.visit(round.index)}
            >
              {t("mentionResume")}
            </Button>
          ) : (
            <Button
              size="sm"
              variant="outline"
              className={buttonClassName}
              disabled={
                disabled ||
                !hasNext ||
                !round.confirmed.has(round.ids[round.index])
              }
              onClick={() => void navigation.visit(round.index + 1)}
            >
              {t("mentionNext")}
            </Button>
          )}
          {showLatest ? null : (
            <Button
              size="sm"
              variant="outline"
              className={buttonClassName}
              disabled={busy}
              onClick={navigation.close}
            >
              {t("mentionClose")}
            </Button>
          )}
        </div>
      ) : navigation.pendingCount > 0 ? (
        <Button
          size="sm"
          variant="outline"
          className={buttonClassName}
          disabled={disabled}
          onClick={() => void navigation.start()}
          aria-label={t("mentionPendingCount", {
            count: navigation.pendingCount,
          })}
        >
          @ {navigation.pendingCount}
        </Button>
      ) : null}
      {showLatest ? (
        <Button
          size="icon-lg"
          variant="ghost"
          className="relative size-12 rounded-full bg-gray-100 text-gray-500 shadow-sm hover:bg-gray-200 hover:text-gray-500 dark:hover:bg-gray-200"
          disabled={busy}
          onClick={onLatest}
          aria-label={latestLabel}
          title={latestLabel}
        >
          <ChevronDownIcon className="size-7 translate-y-[2px]" strokeWidth={1.75} aria-hidden />
          {newCount > 0 ? (
            <span
              className="absolute -top-1 -right-1 min-w-4 rounded-full bg-primary px-1 text-[10px] leading-4 text-primary-foreground tabular-nums"
              aria-hidden
            >
              {newCount}
            </span>
          ) : null}
        </Button>
      ) : null}
    </div>
  )
}
