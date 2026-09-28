/** 客服周期的对客沟通记录：标题行提供在收件箱打开会话，按顺序列出发送方与正文。 */
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"

import { ServiceTranscriptSender, type ServiceTranscriptMessageData } from "@/api"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"

/** 列出周期内的对客沟通；给出 highlightedMessageId 时突出显示该条，打开会话时定位到 focusMessageId，未给出时定位到突出显示的消息。 */
export function ServiceTranscript({
  conversationId,
  messages,
  highlightedMessageId = "",
  focusMessageId = highlightedMessageId,
}: {
  conversationId: string
  messages: ServiceTranscriptMessageData[]
  highlightedMessageId?: string
  focusMessageId?: string
}) {
  const { t } = useTranslation("agents")
  const navigate = useNavigate()

  return (
    <section className="space-y-3">
      <div className="flex items-center justify-between gap-3">
        <h3 className="text-sm font-medium">{t("performance.transcript.title")}</h3>
        <Button
          type="button"
          variant="link"
          size="sm"
          className="h-auto px-0"
          onClick={() => {
            // 在收件箱打开该会话并定位到指定消息。
            const params = new URLSearchParams({ conversation: conversationId })
            if (focusMessageId) params.set("message", focusMessageId)
            navigate(`/inbox?${params.toString()}`)
          }}
        >
          {t("performance.transcript.openConversation")}
        </Button>
      </div>
      <div className="max-h-72 space-y-3 overflow-y-auto rounded-lg border p-3 text-sm">
        {messages.map((message) => (
          <div
            key={message.id}
            className={cn(
              "space-y-0.5 rounded-md px-2 py-1",
              highlightedMessageId && message.id === highlightedMessageId && "bg-muted",
            )}
          >
            <p className="text-xs text-muted-foreground">
              {message.sender === ServiceTranscriptSender.ServiceTranscriptSenderCustomer
                ? t("performance.transcript.customer")
                : message.senderName}
            </p>
            <p className="break-words whitespace-pre-wrap">{message.body}</p>
          </div>
        ))}
      </div>
    </section>
  )
}
