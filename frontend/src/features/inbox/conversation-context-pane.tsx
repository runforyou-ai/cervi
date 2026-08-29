/** 展示当前客户 Conversation 和联系人的只读上下文。 */
import { PanelRightCloseIcon } from "lucide-react"
import { useTranslation } from "react-i18next"

import type { InboxConversation } from "@/api"
import { Button } from "@/components/ui/button"
import { ScrollArea } from "@/components/ui/scroll-area"
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"

/** 渲染上下文栏共用的会话和联系人摘要。 */
function ConversationContextSummary({
  conversation,
  contactName,
  sessionStatus,
}: {
  conversation: InboxConversation
  contactName: string
  sessionStatus: string
}) {
  const { t } = useTranslation("inbox")

  return (
    <div className="divide-y divide-border/70">
      <section className="p-4">
        <h3 className="text-xs font-semibold tracking-wide text-muted-foreground uppercase">
          {t("contextConversation")}
        </h3>
        <dl className="mt-3 grid gap-3 text-sm">
          <div className="grid grid-cols-[5rem_minmax(0,1fr)] gap-3">
            <dt className="text-xs text-muted-foreground">
              {t("contextTitle")}
            </dt>
            <dd className="min-w-0 truncate" title={conversation.title}>
              {conversation.title}
            </dd>
          </div>
          <div className="grid grid-cols-[5rem_minmax(0,1fr)] gap-3">
            <dt className="text-xs text-muted-foreground">
              {t("contextChannel")}
            </dt>
            <dd className="min-w-0 truncate" title={conversation.channelName}>
              {conversation.channelName}
            </dd>
          </div>
          <div className="grid grid-cols-[5rem_minmax(0,1fr)] gap-3">
            <dt className="text-xs text-muted-foreground">
              {t("contextSessionStatus")}
            </dt>
            <dd className="min-w-0 truncate">{sessionStatus}</dd>
          </div>
        </dl>
      </section>
      <section className="p-4">
        <h3 className="text-xs font-semibold tracking-wide text-muted-foreground uppercase">
          {t("contextContact")}
        </h3>
        <dl className="mt-3 grid gap-3 text-sm">
          <div className="grid grid-cols-[5rem_minmax(0,1fr)] gap-3">
            <dt className="text-xs text-muted-foreground">
              {t("contextContactName")}
            </dt>
            <dd className="min-w-0 truncate" title={contactName}>
              {contactName}
            </dd>
          </div>
        </dl>
      </section>
    </div>
  )
}

/** 在宽屏侧栏和较窄视口 Sheet 中复用当前上下文。 */
export function ConversationContextPane({
  conversation,
  contactName,
  sessionStatus,
  desktopVisible,
  sheetOpen,
  onDesktopCollapse,
  onSheetOpenChange,
}: {
  conversation: InboxConversation
  contactName: string
  sessionStatus: string
  desktopVisible: boolean
  sheetOpen: boolean
  onDesktopCollapse: () => void
  onSheetOpenChange: (open: boolean) => void
}) {
  const { t } = useTranslation("inbox")

  return (
    <>
      {desktopVisible ? (
        <aside className="hidden h-full min-h-0 w-80 shrink-0 flex-col border-l bg-background xl:flex">
          <div className="flex h-12 shrink-0 items-center justify-between border-b px-4">
            <h2 className="text-sm font-semibold">{t("contextTitleBar")}</h2>
            <Button
              type="button"
              variant="ghost"
              size="icon-sm"
              aria-label={t("contextClose")}
              title={t("contextClose")}
              onClick={onDesktopCollapse}
            >
              <PanelRightCloseIcon />
            </Button>
          </div>
          <ScrollArea className="min-h-0 flex-1">
            <ConversationContextSummary
              conversation={conversation}
              contactName={contactName}
              sessionStatus={sessionStatus}
            />
          </ScrollArea>
        </aside>
      ) : null}

      <Sheet open={sheetOpen} onOpenChange={onSheetOpenChange}>
        <SheetContent className="data-[side=right]:w-full gap-0 p-0 sm:max-w-sm">
          <SheetHeader className="shrink-0 border-b pr-12">
            <SheetTitle>{t("contextTitleBar")}</SheetTitle>
            <SheetDescription className="sr-only">
              {t("contextDescription")}
            </SheetDescription>
          </SheetHeader>
          <ScrollArea className="min-h-0 flex-1">
            <ConversationContextSummary
              conversation={conversation}
              contactName={contactName}
              sessionStatus={sessionStatus}
            />
          </ScrollArea>
        </SheetContent>
      </Sheet>
    </>
  )
}
