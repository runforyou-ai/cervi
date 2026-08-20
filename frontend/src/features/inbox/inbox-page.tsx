/** 消息会话列表和聊天界面。 */
import { useEffect, useState } from "react"
import {
  MessagesSquareIcon,
  PanelRightCloseIcon,
  PanelRightOpenIcon,
  SearchIcon,
  SendIcon,
} from "lucide-react"
import { useTranslation } from "react-i18next"

import type { Conversation } from "@/api"
import { PageSplit } from "@/components/page-split"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { ScrollArea } from "@/components/ui/scroll-area"
import { Separator } from "@/components/ui/separator"
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"
import { useIsNarrowViewport } from "@/hooks/use-narrow-viewport"
import { cn } from "@/lib/utils"

/** 会话头像和在线状态。 */
function InboxConversationAvatar({ conversation }: { conversation: Conversation }) {
  const { t } = useTranslation("inbox")

  return (
    <div className="relative shrink-0">
      <div className="flex size-9 items-center justify-center rounded-full bg-muted text-xs font-medium text-muted-foreground">
        {conversation.initials}
      </div>
      {conversation.online ? (
        <span
          role="img"
          aria-label={t("onlineIndicator")}
          className="absolute right-0 bottom-0 size-2.5 rounded-full border-2 border-background bg-primary"
        />
      ) : null}
    </div>
  )
}

/** 当前会话的消息列表和输入区。 */
function ConversationThread({
  conversation,
  narrowViewport = false,
  customerPanelOpen,
  onCustomerPanelToggle,
}: {
  conversation: Conversation
  narrowViewport?: boolean
  customerPanelOpen?: boolean
  onCustomerPanelToggle?: () => void
}) {
  const { t } = useTranslation("inbox")

  return (
    <div className="flex h-full min-h-0 flex-col bg-background">
      <header
        className={cn(
          "flex h-14 shrink-0 items-center gap-3 border-b px-4",
          narrowViewport && "pr-14"
        )}
      >
        <InboxConversationAvatar conversation={conversation} />
        <div className="min-w-0">
          <h2 className="truncate text-sm font-medium">{conversation.name}</h2>
          <p className="truncate text-xs text-muted-foreground">
            {conversation.channel} · {conversation.status}
          </p>
        </div>
        {onCustomerPanelToggle ? (
          <Button
            variant="ghost"
            size="icon"
            className="ml-auto"
            aria-label={
              customerPanelOpen
                ? t("customerPanel.hide")
                : t("customerPanel.show")
            }
            aria-pressed={customerPanelOpen}
            title={
              customerPanelOpen
                ? t("customerPanel.hide")
                : t("customerPanel.show")
            }
            onClick={onCustomerPanelToggle}
          >
            {customerPanelOpen ? (
              <PanelRightCloseIcon />
            ) : (
              <PanelRightOpenIcon />
            )}
          </Button>
        ) : null}
      </header>

      <ScrollArea className="min-h-0 flex-1 bg-muted/20">
        <div className="mx-auto flex w-full max-w-3xl flex-col gap-4 p-4 md:p-6">
          <div className="flex items-center gap-3 py-1 text-xs text-muted-foreground">
            <Separator className="flex-1" />
            {t("today")}
            <Separator className="flex-1" />
          </div>
          {conversation.messages.map((message) => (
            <div
              key={message.id}
              aria-label={t(
                message.author === "agent"
                  ? "messageFromAgent"
                  : "messageFromVisitor"
              )}
              className={cn(
                "flex",
                message.author === "agent" ? "justify-end" : "justify-start"
              )}
            >
              <div className="max-w-[82%]">
                <div
                  className={cn(
                    "rounded-2xl px-3.5 py-2.5 text-sm leading-6",
                    message.author === "agent"
                      ? "bg-primary text-primary-foreground"
                      : "border bg-background text-foreground"
                  )}
                >
                  {message.text}
                </div>
                <p
                  className={cn(
                    "mt-1 px-1 text-xs text-muted-foreground",
                    message.author === "agent" && "text-right"
                  )}
                >
                  {message.time}
                </p>
              </div>
            </div>
          ))}
        </div>
      </ScrollArea>

      <div className="shrink-0 border-t bg-background p-3 md:p-4">
        <div className="rounded-2xl border bg-muted/20 p-3">
          <p className="min-h-8 text-sm text-muted-foreground">
            {t("replyPlaceholder")}
          </p>
          <div className="flex items-center justify-between gap-3">
            <span className="text-xs text-muted-foreground">
              {t("replyComingSoon")}
            </span>
            <Button size="sm" disabled>
              <SendIcon />
              {t("send")}
            </Button>
          </div>
        </div>
      </div>
    </div>
  )
}

/** 当前会话的访客资料侧栏。 */
function CustomerPanel({ conversation }: { conversation: Conversation }) {
  const { t } = useTranslation("inbox")

  return (
    <aside className="hidden min-h-0 w-80 shrink-0 flex-col border-l bg-background md:flex">
      <header className="flex h-14 shrink-0 items-center border-b px-4">
        <h2 className="text-sm font-medium">{t("customerPanel.title")}</h2>
      </header>
      <ScrollArea className="min-h-0 flex-1">
        <div className="space-y-6 p-4">
          <div className="flex flex-col items-center gap-3 py-2 text-center">
            <div className="flex size-12 items-center justify-center rounded-full bg-muted text-sm font-medium text-muted-foreground">
              {conversation.initials}
            </div>
            <div>
              <p className="text-sm font-medium">{conversation.name}</p>
              <p className="text-xs text-muted-foreground">
                {conversation.status}
              </p>
            </div>
          </div>
          <Separator />
          <dl className="space-y-4 text-sm">
            <div className="space-y-1">
              <dt className="text-xs text-muted-foreground">
                {t("customerPanel.channel")}
              </dt>
              <dd>{conversation.channel}</dd>
            </div>
            <div className="space-y-1">
              <dt className="text-xs text-muted-foreground">
                {t("customerPanel.notes")}
              </dt>
              <dd className="text-muted-foreground">
                {t("customerPanel.notesPlaceholder")}
              </dd>
            </div>
          </dl>
        </div>
      </ScrollArea>
    </aside>
  )
}

/** 消息会话列表。 */
function InboxConversationList({
  conversations,
  selectedId,
  onSelect,
}: {
  conversations: Conversation[]
  selectedId?: string
  onSelect?: (conversationId: string) => void
}) {
  const { t } = useTranslation("inbox")

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div className="shrink-0 p-3">
        <div className="relative">
          <SearchIcon className="absolute top-2 left-2.5 size-4 text-muted-foreground" />
          <Input
            aria-label={t("search")}
            placeholder={t("search")}
            className="pl-8"
            disabled={conversations.length === 0}
          />
        </div>
      </div>
      {conversations.length === 0 ? null : (
        <ScrollArea className="min-h-0 flex-1">
          <div className="grid gap-1 px-2 pb-3">
            {conversations.map((conversation) => (
              <button
                key={conversation.id}
                type="button"
                aria-pressed={selectedId === conversation.id}
                aria-label={
                  conversation.unread
                    ? `${conversation.name}, ${t("unread", { count: conversation.unread })}`
                    : conversation.name
                }
                className={cn(
                  "flex w-full items-start gap-3 rounded-xl px-3 py-3 text-left transition-colors hover:bg-foreground/6",
                  selectedId === conversation.id && "bg-foreground/12"
                )}
                onClick={() => onSelect?.(conversation.id)}
              >
                <InboxConversationAvatar conversation={conversation} />
                <span className="min-w-0 flex-1">
                  <span className="flex items-center gap-2">
                    <span className="truncate text-sm font-medium">
                      {conversation.name}
                    </span>
                    <span className="ml-auto shrink-0 text-xs text-muted-foreground">
                      {conversation.time}
                    </span>
                  </span>
                  <span className="mt-0.5 flex items-center gap-2">
                    <span className="truncate text-xs text-muted-foreground">
                      {conversation.preview}
                    </span>
                    {conversation.unread ? (
                      <span className="ml-auto flex size-5 shrink-0 items-center justify-center rounded-full bg-primary text-[10px] font-medium text-primary-foreground">
                        {conversation.unread}
                      </span>
                    ) : null}
                  </span>
                </span>
              </button>
            ))}
          </div>
        </ScrollArea>
      )}
    </div>
  )
}

/** 消息会话列表和当前会话。 */
export function InboxPage({
  conversations,
}: {
  conversations: Conversation[]
}) {
  const { t } = useTranslation("inbox")
  const isNarrowViewport = useIsNarrowViewport()
  const [selectedId, setSelectedId] = useState(
    () => conversations[0]?.id ?? ""
  )
  const [isNarrowDetailOpen, setIsNarrowDetailOpen] = useState(false)
  const [isCustomerPanelOpen, setIsCustomerPanelOpen] = useState(
    () => window.matchMedia("(min-width: 1440px)").matches
  )

  useEffect(() => {
    if (!isNarrowViewport) {
      setIsNarrowDetailOpen(false)
    }
  }, [isNarrowViewport])

  useEffect(() => {
    if (!conversations.some((conversation) => conversation.id === selectedId)) {
      setSelectedId(conversations[0]?.id ?? "")
    }
  }, [conversations, selectedId])

  if (conversations.length === 0) {
    return (
      <PageSplit
        paneWidth="lg"
        className="bg-background"
        pane={
          <InboxConversationList conversations={conversations} />
        }
        mainClassName="items-center justify-center p-6"
      >
        <div className="max-w-sm text-center">
          <div className="mx-auto mb-4 flex size-11 items-center justify-center rounded-xl border bg-background shadow-sm">
            <MessagesSquareIcon className="size-5 text-muted-foreground" />
          </div>
          <h2 className="text-base font-semibold tracking-tight">
            {t("emptyTitle")}
          </h2>
          <p className="mt-2 text-sm text-muted-foreground">
            {t("emptyDescription")}
          </p>
        </div>
      </PageSplit>
    )
  }

  const selectedConversation =
    conversations.find((conversation) => conversation.id === selectedId) ??
    conversations[0]

  /** 选中一个会话。 */
  function selectConversation(conversationId: string) {
    setSelectedId(conversationId)

    if (isNarrowViewport) {
      setIsNarrowDetailOpen(true)
    }
  }

  return (
    <>
      <PageSplit
        paneWidth="lg"
        paneOnNarrow="fill"
        className="overflow-x-auto overflow-y-hidden bg-background"
        pane={
          <InboxConversationList
            conversations={conversations}
            selectedId={selectedId}
            onSelect={selectConversation}
          />
        }
        mainClassName={cn(
          "hidden md:flex md:flex-row",
          isCustomerPanelOpen ? "min-w-[880px]" : "min-w-[560px]",
        )}
      >
        <section className="min-h-0 min-w-[560px] flex-1">
          <ConversationThread
            conversation={selectedConversation}
            customerPanelOpen={isCustomerPanelOpen}
            onCustomerPanelToggle={() =>
              setIsCustomerPanelOpen((isOpen) => !isOpen)
            }
          />
        </section>

        {isCustomerPanelOpen ? (
          <CustomerPanel conversation={selectedConversation} />
        ) : null}
      </PageSplit>

      <Sheet open={isNarrowDetailOpen} onOpenChange={setIsNarrowDetailOpen}>
        <SheetContent className="data-[side=right]:w-full p-0 sm:max-w-lg">
          <SheetHeader className="sr-only">
            <SheetTitle>
              {t("conversationTitle", { name: selectedConversation.name })}
            </SheetTitle>
            <SheetDescription>{t("detailDescription")}</SheetDescription>
          </SheetHeader>
          <ConversationThread
            conversation={selectedConversation}
            narrowViewport
          />
        </SheetContent>
      </Sheet>
    </>
  )
}
