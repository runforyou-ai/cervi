import type { TFunction } from "i18next"
import { useEffect, useMemo, useState } from "react"
import {
  PanelRightCloseIcon,
  PanelRightOpenIcon,
  SearchIcon,
  SendIcon,
} from "lucide-react"
import { useTranslation } from "react-i18next"

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
import { useIsMobile } from "@/hooks/use-mobile"
import { cn } from "@/lib/utils"

type Conversation = {
  id: string
  name: string
  initials: string
  channel: string
  preview: string
  time: string
  status: string
  unread?: number
  online?: boolean
  messages: {
    id: string
    author: "visitor" | "agent"
    text: string
    time: string
  }[]
}

function createConversations(t: TFunction<"inbox">): Conversation[] {
  return [
  {
    id: "lin-xiao",
    name: t("conversations.linXiao.name"),
    initials: t("conversations.linXiao.initials"),
    channel: t("channels.webChat"),
    preview: t("conversations.linXiao.preview"),
    time: t("conversations.linXiao.time"),
    status: t("conversations.linXiao.status"),
    unread: 2,
    online: true,
    messages: [
      {
        id: "lin-1",
        author: "visitor",
        text: t("conversations.linXiao.messages.lin1"),
        time: "10:41",
      },
      {
        id: "lin-2",
        author: "agent",
        text: t("conversations.linXiao.messages.lin2"),
        time: "10:42",
      },
      {
        id: "lin-3",
        author: "visitor",
        text: t("conversations.linXiao.messages.lin3"),
        time: "10:43",
      },
    ],
  },
  {
    id: "chen-yu",
    name: t("conversations.chenYu.name"),
    initials: t("conversations.chenYu.initials"),
    channel: t("channels.inApp"),
    preview: t("conversations.chenYu.preview"),
    time: t("conversations.chenYu.time"),
    status: t("conversations.chenYu.status"),
    messages: [
      {
        id: "chen-1",
        author: "visitor",
        text: t("conversations.chenYu.messages.chen1"),
        time: "10:28",
      },
      {
        id: "chen-2",
        author: "agent",
        text: t("conversations.chenYu.messages.chen2"),
        time: "10:31",
      },
      {
        id: "chen-3",
        author: "visitor",
        text: t("conversations.chenYu.messages.chen3"),
        time: "10:34",
      },
    ],
  },
  {
    id: "zhou-ran",
    name: t("conversations.zhouRan.name"),
    initials: t("conversations.zhouRan.initials"),
    channel: t("channels.webChat"),
    preview: t("conversations.zhouRan.preview"),
    time: t("conversations.zhouRan.time"),
    status: t("conversations.zhouRan.status"),
    unread: 1,
    online: true,
    messages: [
      {
        id: "zhou-1",
        author: "visitor",
        text: t("conversations.zhouRan.messages.zhou1"),
        time: "10:12",
      },
      {
        id: "zhou-2",
        author: "agent",
        text: t("conversations.zhouRan.messages.zhou2"),
        time: "10:13",
      },
    ],
  },
  {
    id: "alex",
    name: t("conversations.alex.name"),
    initials: t("conversations.alex.initials"),
    channel: t("channels.inApp"),
    preview: t("conversations.alex.preview"),
    time: t("conversations.alex.time"),
    status: t("conversations.alex.status"),
    messages: [
      {
        id: "alex-1",
        author: "visitor",
        text: t("conversations.alex.messages.alex1"),
        time: "09:35",
      },
      {
        id: "alex-2",
        author: "agent",
        text: t("conversations.alex.messages.alex2"),
        time: "09:38",
      },
    ],
  },
  ]
}

function ConversationAvatar({ conversation }: { conversation: Conversation }) {
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

function ConversationThread({
  conversation,
  mobile = false,
  customerPanelOpen,
  onCustomerPanelToggle,
}: {
  conversation: Conversation
  mobile?: boolean
  customerPanelOpen?: boolean
  onCustomerPanelToggle?: () => void
}) {
  const { t } = useTranslation("inbox")

  return (
    <div className="flex h-full min-h-0 flex-col bg-background">
      <header
        className={cn(
          "flex h-14 shrink-0 items-center gap-3 border-b px-4",
          mobile && "pr-14"
        )}
      >
        <ConversationAvatar conversation={conversation} />
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

function CustomerPanel({ conversation }: { conversation: Conversation }) {
  const { t } = useTranslation("inbox")

  return (
    <aside className="hidden min-h-0 w-[320px] shrink-0 flex-col border-l bg-background md:flex">
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

export function InboxPage() {
  const { t } = useTranslation("inbox")
  const isMobile = useIsMobile()
  const conversations = useMemo(() => createConversations(t), [t])
  const [selectedId, setSelectedId] = useState(conversations[0].id)
  const [isMobileDetailOpen, setIsMobileDetailOpen] = useState(false)
  const [isCustomerPanelOpen, setIsCustomerPanelOpen] = useState(
    () => window.innerWidth >= 1440
  )
  const selectedConversation =
    conversations.find((conversation) => conversation.id === selectedId) ??
    conversations[0]

  useEffect(() => {
    if (!isMobile) {
      setIsMobileDetailOpen(false)
    }
  }, [isMobile])

  function selectConversation(conversationId: string) {
    setSelectedId(conversationId)

    if (isMobile) {
      setIsMobileDetailOpen(true)
    }
  }

  return (
    <div className="flex min-h-0 flex-1 overflow-x-auto overflow-y-hidden bg-background">
      <section className="flex min-h-0 w-full shrink-0 flex-col border-r md:w-[320px]">
        <div className="flex h-14 shrink-0 items-center px-4">
          <div>
            <h2 className="text-sm font-medium">{t("title")}</h2>
            <p className="text-xs text-muted-foreground">
              {t("ongoing", { count: conversations.length })}
            </p>
          </div>
        </div>
        <Separator />
        <div className="shrink-0 p-3">
          <div className="relative">
            <SearchIcon className="absolute top-2 left-2.5 size-4 text-muted-foreground" />
            <Input
              aria-label={t("search")}
              placeholder={t("search")}
              className="pl-8"
            />
          </div>
        </div>
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
                  "flex w-full items-start gap-3 rounded-xl px-3 py-3 text-left transition-colors hover:bg-muted/60",
                  selectedId === conversation.id && "bg-muted"
                )}
                onClick={() => selectConversation(conversation.id)}
              >
                <ConversationAvatar conversation={conversation} />
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
      </section>

      <section className="hidden min-w-[560px] flex-1 md:block">
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

      <Sheet open={isMobileDetailOpen} onOpenChange={setIsMobileDetailOpen}>
        <SheetContent className="data-[side=right]:w-full p-0 sm:max-w-lg">
          <SheetHeader className="sr-only">
            <SheetTitle>
              {t("conversationTitle", { name: selectedConversation.name })}
            </SheetTitle>
            <SheetDescription>{t("detailDescription")}</SheetDescription>
          </SheetHeader>
          <ConversationThread conversation={selectedConversation} mobile />
        </SheetContent>
      </Sheet>
    </div>
  )
}
