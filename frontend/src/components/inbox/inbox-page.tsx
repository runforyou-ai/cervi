import { useEffect, useState } from "react"
import { SearchIcon, SendIcon } from "lucide-react"

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

const conversations: Conversation[] = [
  {
    id: "lin-xiao",
    name: "林晓",
    initials: "林",
    channel: "网站聊天",
    preview: "退款大概多久到账？",
    time: "刚刚",
    status: "在线",
    unread: 2,
    online: true,
    messages: [
      {
        id: "lin-1",
        author: "visitor",
        text: "你好，我昨天提交了退款申请。",
        time: "10:41",
      },
      {
        id: "lin-2",
        author: "agent",
        text: "你好，退款申请已经审核通过，款项正在原路退回。",
        time: "10:42",
      },
      {
        id: "lin-3",
        author: "visitor",
        text: "退款大概多久到账？",
        time: "10:43",
      },
    ],
  },
  {
    id: "chen-yu",
    name: "陈语",
    initials: "陈",
    channel: "应用内消息",
    preview: "已经收到，谢谢你们。",
    time: "8 分钟前",
    status: "8 分钟前活跃",
    messages: [
      {
        id: "chen-1",
        author: "visitor",
        text: "请问发票已经开好了吗？",
        time: "10:28",
      },
      {
        id: "chen-2",
        author: "agent",
        text: "已经发送到你的账户邮箱，请注意查收。",
        time: "10:31",
      },
      {
        id: "chen-3",
        author: "visitor",
        text: "已经收到，谢谢你们。",
        time: "10:34",
      },
    ],
  },
  {
    id: "zhou-ran",
    name: "周然",
    initials: "周",
    channel: "网站聊天",
    preview: "登录页面一直提示网络错误。",
    time: "25 分钟前",
    status: "在线",
    unread: 1,
    online: true,
    messages: [
      {
        id: "zhou-1",
        author: "visitor",
        text: "登录页面一直提示网络错误。",
        time: "10:12",
      },
      {
        id: "zhou-2",
        author: "agent",
        text: "收到，我先帮你确认一下当前的服务状态。",
        time: "10:13",
      },
    ],
  },
  {
    id: "alex",
    name: "Alex Morgan",
    initials: "AM",
    channel: "应用内消息",
    preview: "想了解一下团队版的价格。",
    time: "1 小时前",
    status: "1 小时前活跃",
    messages: [
      {
        id: "alex-1",
        author: "visitor",
        text: "你好，想了解一下团队版的价格。",
        time: "09:35",
      },
      {
        id: "alex-2",
        author: "agent",
        text: "你好，团队版会根据席位数量计费，后续我可以为你介绍详细方案。",
        time: "09:38",
      },
    ],
  },
]

function ConversationAvatar({ conversation }: { conversation: Conversation }) {
  return (
    <div className="relative shrink-0">
      <div className="flex size-9 items-center justify-center rounded-full bg-muted text-xs font-medium text-muted-foreground">
        {conversation.initials}
      </div>
      {conversation.online ? (
        <span className="absolute right-0 bottom-0 size-2.5 rounded-full border-2 border-background bg-primary" />
      ) : null}
    </div>
  )
}

function ConversationThread({
  conversation,
  mobile = false,
}: {
  conversation: Conversation
  mobile?: boolean
}) {
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
      </header>

      <ScrollArea className="min-h-0 flex-1 bg-muted/20">
        <div className="mx-auto flex w-full max-w-3xl flex-col gap-4 p-4 md:p-6">
          <div className="flex items-center gap-3 py-1 text-xs text-muted-foreground">
            <Separator className="flex-1" />
            今天
            <Separator className="flex-1" />
          </div>
          {conversation.messages.map((message) => (
            <div
              key={message.id}
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
          <p className="min-h-8 text-sm text-muted-foreground">输入回复…</p>
          <div className="flex items-center justify-between gap-3">
            <span className="text-xs text-muted-foreground">
              回复功能将在后续接入
            </span>
            <Button size="sm" disabled>
              <SendIcon />
              发送
            </Button>
          </div>
        </div>
      </div>
    </div>
  )
}

export function InboxPage() {
  const isMobile = useIsMobile()
  const [selectedId, setSelectedId] = useState(conversations[0].id)
  const [isMobileDetailOpen, setIsMobileDetailOpen] = useState(false)
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
    <div className="flex min-h-0 flex-1 overflow-hidden bg-background">
      <section className="flex min-h-0 w-full shrink-0 flex-col border-r md:w-80 lg:w-[22rem]">
        <div className="flex h-14 shrink-0 items-center px-4">
          <div>
            <h2 className="text-sm font-medium">客户会话</h2>
            <p className="text-xs text-muted-foreground">4 个进行中</p>
          </div>
        </div>
        <Separator />
        <div className="shrink-0 p-3">
          <div className="relative">
            <SearchIcon className="absolute top-2 left-2.5 size-4 text-muted-foreground" />
            <Input aria-label="搜索会话" placeholder="搜索会话" className="pl-8" />
          </div>
        </div>
        <ScrollArea className="min-h-0 flex-1">
          <div className="grid gap-1 px-2 pb-3">
            {conversations.map((conversation) => (
              <button
                key={conversation.id}
                type="button"
                aria-pressed={selectedId === conversation.id}
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

      <section className="hidden min-w-0 flex-1 md:block">
        <ConversationThread conversation={selectedConversation} />
      </section>

      <Sheet open={isMobileDetailOpen} onOpenChange={setIsMobileDetailOpen}>
        <SheetContent className="data-[side=right]:w-full p-0 sm:max-w-lg">
          <SheetHeader className="sr-only">
            <SheetTitle>与 {selectedConversation.name} 的会话</SheetTitle>
            <SheetDescription>客服会话详情</SheetDescription>
          </SheetHeader>
          <ConversationThread conversation={selectedConversation} mobile />
        </SheetContent>
      </Sheet>
    </div>
  )
}
