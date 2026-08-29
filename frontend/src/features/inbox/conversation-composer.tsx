/** 建立客户会话回复区的非交互布局。 */
import { PaperclipIcon } from "lucide-react"
import { useTranslation } from "react-i18next"

import { Button } from "@/components/ui/button"
import { Textarea } from "@/components/ui/textarea"

/** 展示整体禁用的回复编辑区。 */
export function ConversationComposer({
  conversationID,
}: {
  conversationID: string
}) {
  const { t } = useTranslation("inbox")
  const inputID = `conversation-reply-${conversationID}`

  return (
    <footer
      data-slot="conversation-composer"
      data-conversation-id={conversationID}
      className="shrink-0 border-t border-border/60 bg-background p-3"
    >
      <div className="overflow-hidden rounded-xl border border-input bg-muted/15 shadow-xs">
        <label
          htmlFor={inputID}
          className="block px-3 pt-2.5 text-xs font-medium text-muted-foreground"
        >
          {t("replyLabel")}
        </label>
        <Textarea
          id={inputID}
          disabled
          rows={3}
          className="min-h-20 resize-none rounded-none border-0 bg-transparent py-2 shadow-none focus-visible:ring-0"
        />
        <div className="flex items-center justify-between gap-3 px-2.5 pb-2.5">
          <Button
            type="button"
            variant="ghost"
            size="icon-sm"
            disabled
            aria-label={t("attachmentAdd")}
            title={t("attachmentAdd")}
          >
            <PaperclipIcon />
          </Button>
          <Button type="button" size="sm" disabled>
            {t("messageSend")}
          </Button>
        </div>
      </div>
    </footer>
  )
}
