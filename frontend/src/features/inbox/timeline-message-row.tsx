/** 时间线中的单条消息行：日期分隔、客服处理周期分隔，以及系统事件或消息气泡。 */
import { useTranslation } from "react-i18next"

import { MessageType, ServiceSessionStatus } from "@/api"

import {
  formatMessageTime,
  formatTimelineDayLabel,
  messagesShareGroup,
} from "./timeline-grouping"
import {
  TimelineMessageBubble,
  type TimelineMessageBubbleContext,
} from "./timeline-message-bubble"
import type { TimelineMessage } from "./timeline-messages"
import { formatSystemEvent } from "./timeline-system-event"

/** 按相邻消息决定分隔与分组，展示一条时间线消息。 */
export function TimelineMessageRow({
  message,
  previous,
  next,
  ...context
}: TimelineMessageBubbleContext & {
  message: TimelineMessage
  previous: TimelineMessage | undefined
  next: TimelineMessage | undefined
}) {
  const { t } = useTranslation(["inbox", "common"])
  const { formatters } = context
  const currentIdentityID = context.currentUser.identityId
  const date = new Date(message.originatedAt)
  const day = formatters.dayKey.format(date)
  const startsDay =
    !previous ||
    formatters.dayKey.format(
      new Date(previous.originatedAt),
    ) !== day
  const systemEvent = message.systemEvent
  const systemEventText = systemEvent
    ? formatSystemEvent(systemEvent, currentIdentityID, t)
    : null

  return (
    <div>
      {startsDay ? (
        <div className="my-3 flex items-center gap-2.5 text-xs font-medium text-muted-foreground">
          <span className="h-px flex-1 bg-border/50" />
          <time dateTime={day}>{formatTimelineDayLabel(formatters, date, t)}</time>
          <span className="h-px flex-1 bg-border/50" />
        </div>
      ) : null}
      {message.sessionStart ? (
        <div className="my-3 text-center text-xs font-semibold text-muted-foreground">
          <span>
            {t("sessionBoundary", {
              sequence: message.sessionStart.sequence,
              time: formatMessageTime(
                formatters.sessionTime,
                new Date(message.sessionStart.startedAt),
              ),
            })}{" "}
            ·{" "}
            {message.sessionStart.status ===
            ServiceSessionStatus.ServiceSessionStatusClosed
              ? t("sessionBoundaryClosed")
              : t("sessionBoundaryOngoing")}
          </span>
        </div>
      ) : null}
      {message.type === MessageType.MessageTypeSystem &&
      systemEventText ? (
        <div
          data-message-id={message.local ? undefined : message.id}
          tabIndex={-1}
          className="my-3 flex flex-col items-center gap-1 px-10 text-center text-xs text-muted-foreground"
        >
          <span title={formatters.full.format(date)}>
            {systemEventText}
          </span>
          {systemEvent?.reasonText ? (
            <span className="max-w-full break-words">
              {systemEvent.reasonText}
            </span>
          ) : null}
          {systemEvent?.ratingComment ? (
            <span className="max-w-full whitespace-pre-wrap break-words">
              {systemEvent.ratingComment}
            </span>
          ) : null}
        </div>
      ) : (
        <TimelineMessageBubble
          {...context}
          message={message}
          date={date}
          spaced={Boolean(previous)}
          startsGroup={!messagesShareGroup(previous, message, currentIdentityID, formatters.dayKey)}
          endsGroup={!messagesShareGroup(message, next, currentIdentityID, formatters.dayKey)}
        />
      )}
    </div>
  )
}
