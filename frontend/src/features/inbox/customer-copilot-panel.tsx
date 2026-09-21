/** 客户会话右侧栏的 AI 助手：切换和新建 Copilot 线程，在线程中提问并把 AI 回复填入对客草稿。 */
import { useEffect, useRef, useState, type RefObject } from "react"
import { ChevronDownIcon } from "lucide-react"
import { useTranslation } from "react-i18next"

import {
  ConversationType,
  listCustomerCopilotThreads,
  listCustomerReplyAgents,
  sendFirstCustomerCopilotMessage,
  type ConversationMessageData,
  type ConversationMessageReference,
  type CustomerCopilotThread,
  type DirectTextMessageInput,
} from "@/api"
import { LoadingIndicator } from "@/components/loading-indicator"
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { NativeSelect } from "@/components/ui/native-select"
import { useWorkspace } from "@/contexts/workspace-context"
import {
  ConversationComposer,
  type ComposerDraftBridge,
} from "@/features/inbox/conversation-composer"
import { ConversationTimeline } from "@/features/inbox/conversation-timeline"
import { useOutgoingMessages } from "@/features/inbox/outgoing-message-context"
import type { OutgoingConversationDraft } from "@/features/inbox/outgoing-message-store"
import { useConversationTime, useMinuteTick } from "@/features/inbox/use-conversation-time"
import {
  memberChatPollingInterval,
  useMemberChatPollingActive,
} from "@/features/inbox/use-member-chat-polling"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource, useResourceInvalidator } from "@/hooks/use-resource"
import { cn } from "@/lib/utils"

/** 本页内各客户会话最近查看的线程，页面刷新后回到最近更新的线程。 */
const lastViewedThreads = new Map<string, string>()

/** 展示客户会话的 Copilot 线程，没有线程或新建对话时直接展示 AI 员工选择与提问输入框。 */
export function CustomerCopilotPanel({
  customerConversationID,
  replyDisabledReason,
  customerDraftRef,
  active,
}: {
  customerConversationID: string
  replyDisabledReason: string | null
  customerDraftRef: RefObject<ComposerDraftBridge | null>
  active: boolean
}) {
  const { t } = useTranslation(["inbox", "common"])
  const { identity } = useWorkspace()
  const invalidate = useResourceInvalidator()
  const pollingActive = useMemberChatPollingActive()
  const aliveRef = useRef(true)
  const threadsKey = resourceKeys.customerCopilotThreads(customerConversationID)
  const threadsResource = useResource(
    threadsKey,
    () => listCustomerCopilotThreads(customerConversationID),
    // 变更通知按线程会话编号失效线程消息，线程列表按所属客户会话存放，页签打开期间按固定间隔重读。
    { staleTime: 0, refetchInterval: active && pollingActive ? memberChatPollingInterval : false },
  )
  const threads = threadsResource.data ?? []
  const [selectedID, setSelectedID] = useState(() => lastViewedThreads.get(customerConversationID) ?? "")
  const [composingNew, setComposingNew] = useState(false)
  const [draftID, setDraftID] = useState(() => window.crypto.randomUUID())
  const [pendingReply, setPendingReply] = useState<string | null>(null)
  const appliedRef = useRef(false)
  const storageKey = `cervi.inbox.copilot.${identity.user.identityId}`
  const [preferredAgentID, setPreferredAgentID] = useState(() => {
    // 读取本人上次新建对话选择的 AI 员工，本机存储不可用时不预选。
    try {
      return localStorage.getItem(storageKey) ?? ""
    } catch {
      return ""
    }
  })

  useEffect(() => {
    aliveRef.current = true
    return () => {
      aliveRef.current = false
    }
  }, [])

  // 优先展示本页上次查看且仍存在的线程，否则展示最近更新的线程；没有线程时进入新对话。
  const selected = composingNew
    ? null
    : (threads.find((thread) => thread.id === selectedID) ?? threads[0] ?? null)

  useEffect(() => {
    // 首次进入或所选线程消失时固定到实际展示的线程，列表顺序变化不再改变当前视图。
    if (!selected || selected.id === selectedID) return
    lastViewedThreads.set(customerConversationID, selected.id)
    setSelectedID(selected.id)
  }, [customerConversationID, selected, selectedID])
  const drafting = threadsResource.data !== undefined && !selected
  const agentOptions = useResource(
    resourceKeys.customerReplyAgents(),
    listCustomerReplyAgents,
    { enabled: drafting && active, staleTime: 0 },
  )
  const agents = agentOptions.data ?? []
  const agentIdentityID = agents.some((agent) => agent.identityId === preferredAgentID)
    ? preferredAgentID
    : (agents[0]?.identityId ?? "")

  /** 切换到指定线程并记住本页的查看位置。 */
  function openThread(threadID: string) {
    lastViewedThreads.set(customerConversationID, threadID)
    setSelectedID(threadID)
    setComposingNew(false)
  }

  /** 新对话首条提问或附件保存后，刷新线程列表并切换到该线程。 */
  async function openCreatedThread(threadID: string) {
    lastViewedThreads.set(customerConversationID, threadID)
    await invalidate(threadsKey)
    if (aliveRef.current) openThread(threadID)
  }

  /** 按草稿是否为空直接填入或确认后替换对客回复草稿。 */
  function applyReply(body: string) {
    const bridge = customerDraftRef.current
    if (!bridge) return
    if (bridge.read().trim()) {
      setPendingReply(body)
      return
    }
    bridge.replace(body)
  }

  if (threadsResource.data === undefined) {
    return threadsResource.error ? (
      <div className="flex flex-1 flex-col items-center justify-center gap-3 p-6 text-center">
        <p className="text-sm text-muted-foreground">{t("copilotThreadsLoadError")}</p>
        <Button size="sm" variant="outline" onClick={() => void threadsResource.refresh()}>
          {t("common:actions.retry")}
        </Button>
      </div>
    ) : (
      <LoadingIndicator className="flex-1 justify-center">{t("common:status.loading")}</LoadingIndicator>
    )
  }

  const threadID = selected?.id ?? draftID
  const draftDisabledReason = agentOptions.error
    ? t("replyAssistantAgentsLoadError")
    : agentOptions.data !== undefined && agents.length === 0
      ? t("agentPickerEmpty")
      : null

  return (
    <>
      {threads.length > 0 ? (
        <div className="flex shrink-0 items-center gap-2 px-3 py-2">
          <CopilotThreadPicker
            threads={threads}
            selected={selected}
            onSelect={openThread}
          />
          <Button
            type="button"
            size="sm"
            variant="outline"
            className="shrink-0"
            disabled={drafting}
            onClick={() => {
              setDraftID(window.crypto.randomUUID())
              setComposingNew(true)
            }}
          >
            {t("copilotNewConversation")}
          </Button>
        </div>
      ) : null}
      {drafting ? (
        <div className="flex shrink-0 items-center gap-2 border-b px-3 py-2">
          <label htmlFor={`copilot-agent-${customerConversationID}`} className="shrink-0 text-xs text-muted-foreground">
            {t("copilotAgent")}
          </label>
          <NativeSelect
            id={`copilot-agent-${customerConversationID}`}
            className="h-8 min-w-0 flex-1 text-xs shadow-none"
            value={agentIdentityID}
            disabled={agents.length === 0}
            onChange={(event) => {
              const value = event.target.value
              setPreferredAgentID(value)
              try {
                localStorage.setItem(storageKey, value)
              } catch {
                // 本机存储不可用时只在当前页面保留选择。
              }
            }}
          >
            {agents.map((agent) => (
              <option key={agent.identityId} value={agent.identityId}>
                {agent.displayName}
              </option>
            ))}
          </NativeSelect>
        </div>
      ) : null}
      <CopilotThreadView
        key={threadID}
        threadID={threadID}
        thread={selected}
        disabledReason={selected ? (selected.agentActive ? null : t("copilotAgentUnavailable")) : draftDisabledReason}
        active={active}
        applyReplyDisabledReason={replyDisabledReason}
        onApplyReply={applyReply}
        onSent={() => void invalidate(threadsKey)}
        sendFirstMessage={
          selected
            ? undefined
            : async (input) => {
                const result = await sendFirstCustomerCopilotMessage(customerConversationID, {
                  threadId: threadID,
                  agentIdentityId: agentIdentityID,
                  clientMessageId: input.clientMessageId,
                  body: input.body,
                })
                await openCreatedThread(result.thread.id)
                return result.message
              }
        }
        attachmentDraft={
          selected
            ? undefined
            : { conversationID: threadID, agentIdentityID, customerConversationID }
        }
        onAttachmentThreadCreated={(threadConversationID) => void openCreatedThread(threadConversationID)}
      />
      <AlertDialog open={pendingReply !== null} onOpenChange={(open) => !open && setPendingReply(null)}>
        <AlertDialogContent
          onCloseAutoFocus={(event) => {
            // 替换草稿后焦点交给回复输入框。
            if (!appliedRef.current) return
            appliedRef.current = false
            event.preventDefault()
          }}
        >
          <AlertDialogHeader>
            <AlertDialogTitle>{t("copilotApplyReplyConfirmTitle")}</AlertDialogTitle>
            <AlertDialogDescription>{t("copilotApplyReplyConfirmDescription")}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t("common:actions.cancel")}</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => {
                if (pendingReply === null) return
                appliedRef.current = true
                customerDraftRef.current?.replace(pendingReply)
                setPendingReply(null)
              }}
            >
              {t("common:actions.confirm")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}

/** 以首条提问为主文字列出线程，并显示 AI 员工、创建人和最近更新时间。 */
function CopilotThreadPicker({
  threads,
  selected,
  onSelect,
}: {
  threads: CustomerCopilotThread[]
  selected: CustomerCopilotThread | null
  onSelect: (threadID: string) => void
}) {
  const { t } = useTranslation("inbox")
  const formatTime = useConversationTime()
  useMinuteTick()
  const threadMeta = (thread: CustomerCopilotThread) =>
    t("copilotThreadMeta", {
      agent: thread.agentName,
      creator: thread.createdByName,
      time: formatTime(thread.lastActivityAt),
    })

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          type="button"
          size="sm"
          variant="ghost"
          className="h-auto min-h-8 min-w-0 flex-1 justify-between gap-2 px-2 py-1 text-left"
          aria-label={t("copilotThreadPicker")}
        >
          <span className="min-w-0">
            <span className={cn("block truncate text-sm", selected ? "font-medium" : "font-normal text-muted-foreground")}>
              {selected?.title ?? t("copilotThreadPicker")}
            </span>
            {selected ? (
              <span className="block truncate text-xs font-normal text-muted-foreground">
                {threadMeta(selected)}
              </span>
            ) : null}
          </span>
          <ChevronDownIcon className="shrink-0 text-muted-foreground" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="max-h-80 w-[var(--radix-dropdown-menu-trigger-width)] min-w-64 overflow-y-auto">
        {threads.map((thread) => (
          <DropdownMenuItem
            key={thread.id}
            className="flex-col items-start gap-0.5"
            onSelect={() => onSelect(thread.id)}
          >
            <span className="w-full truncate text-sm">{thread.title}</span>
            <span className="w-full truncate text-xs text-muted-foreground">{threadMeta(thread)}</span>
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

/** 展示单个线程或新对话的时间线与提问输入框。 */
function CopilotThreadView({
  threadID,
  thread,
  active,
  disabledReason,
  applyReplyDisabledReason,
  onApplyReply,
  onSent,
  sendFirstMessage,
  attachmentDraft,
  onAttachmentThreadCreated,
}: {
  threadID: string
  thread: CustomerCopilotThread | null
  active: boolean
  disabledReason: string | null
  applyReplyDisabledReason: string | null
  onApplyReply: (body: string) => void
  onSent: () => void
  sendFirstMessage?: (input: DirectTextMessageInput) => Promise<ConversationMessageData>
  attachmentDraft?: { conversationID: string; agentIdentityID: string; customerConversationID: string }
  onAttachmentThreadCreated: (threadConversationID: string) => void
}) {
  const { identity } = useWorkspace()
  const prepareSendRef = useRef<(() => Promise<boolean>) | null>(null)
  const outgoing = useOutgoingMessages(threadID)
  const [replyTo, setReplyTo] = useState<ConversationMessageReference | null>(null)
  const [retryDraft, setRetryDraft] = useState<OutgoingConversationDraft | null>(null)

  return (
    <>
      <ConversationTimeline
        prepareSendRef={prepareSendRef}
        conversationID={threadID}
        conversationType={ConversationType.ConversationTypeCopilot}
        currentUser={identity.user}
        outgoingMessages={outgoing.messages}
        onRetryFailedMessage={setRetryDraft}
        retryFailedMessageDisabled={Boolean(disabledReason)}
        onReplyMessage={thread && !disabledReason ? setReplyTo : undefined}
        mentionNavigation={false}
        enabled={Boolean(thread) && active}
        onApplyReply={onApplyReply}
        applyReplyDisabledReason={applyReplyDisabledReason}
      />
      <ConversationComposer
        conversationID={thread ? threadID : ""}
        conversationType={ConversationType.ConversationTypeCopilot}
        disabledReason={disabledReason}
        onBeforeSend={() => prepareSendRef.current?.() ?? Promise.resolve(true)}
        submitOnEnter
        refocusAfterSubmit
        retryFailedMessage
        retryDraft={retryDraft}
        replyTo={replyTo}
        currentIdentityID={identity.user.identityId}
        onRetryDraftHandled={() => setRetryDraft(null)}
        onReplyToChange={setReplyTo}
        onSending={outgoing.start}
        onSent={outgoing.succeed}
        onFailed={outgoing.fail}
        onSucceeded={onSent}
        sendIndividualMessage={sendFirstMessage}
        attachmentAgentDraft={attachmentDraft}
        onAttachmentConversationCreated={(_, conversationID) => onAttachmentThreadCreated(conversationID)}
      />
    </>
  )
}
