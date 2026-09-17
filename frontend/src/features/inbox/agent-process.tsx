/** 展示 Agent 运行摘要与运行中的实时过程，展开时读取思考过程与工具详情，并提供停止回复入口。 */
import { useEffect, useLayoutEffect, useRef, useState } from "react"
import { useNavigate } from "react-router"
import { toast } from "sonner"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource, useResourceInvalidator } from "@/hooks/use-resource"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"
import { BrainIcon, ChevronDownIcon, LightbulbIcon, SquareIcon } from "lucide-react"
import { MessageMarkdown } from "@/components/message-markdown"
import { ProfileAvatar } from "@/components/profile-avatar"
import { resolveAppPlatform } from "@/platform/app-platform"
import { openExternalURL } from "@/platform/external-navigation"
import { useTranslation } from "react-i18next"
import { Popover } from "radix-ui"

import { useAgentRunStream } from "./use-agent-run-stream"
import {
  isApiError,
  getAgentRunProcess,
  stopAgentReply,
  stopCustomerCopilotReply,
  stopGroupAgentReply,
  AgentRunBlockKind,
  AgentRunStatus,
  AgentToolCallStatus,
  type AgentToolCall,
  type ConversationAgentProcessData,
  type ConversationAgentRun,
  type ConversationPendingAgent,
  type RunStreamState,
  type RunStreamToolCall,
} from "@/api"
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@/components/ui/collapsible"
import { usePortalContainer } from "@/components/ui/portal-container"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { cn } from "@/lib/utils"

/** 在截断末尾提供更多按钮，点击后浮层展示完整原文。 */
function ToolValue({ value }: { value: string }) {
  const { t } = useTranslation("inbox")
  const pagePortal = usePortalContainer()
  const element = useRef<HTMLPreElement>(null)
  const [truncated, setTruncated] = useState(false)
  useLayoutEffect(() => {
    const node = element.current
    if (!node) return
    const measure = () => setTruncated(
      node.scrollHeight > node.clientHeight || node.scrollWidth > node.clientWidth,
    )
    measure()
    const observer = new ResizeObserver(measure)
    observer.observe(node)
    return () => observer.disconnect()
  }, [value])

  return (
    <div className="relative min-w-0">
      <pre
        ref={element}
        className="max-h-20 overflow-hidden whitespace-pre-wrap break-all font-mono text-xs leading-5"
      >
        {value}
      </pre>
      {truncated && (pagePortal?.active ?? true) ? (
        <Popover.Root>
          <span className="absolute right-0 bottom-0 flex items-center gap-1 bg-muted pl-1 text-xs leading-5">
            <span aria-hidden="true">…</span>
            <Popover.Trigger asChild>
              <button
                type="button"
                className="rounded-sm text-muted-foreground hover:underline focus-visible:outline focus-visible:outline-ring"
              >
                {t("agentToolMore")}
              </button>
            </Popover.Trigger>
          </span>
          <Popover.Portal container={pagePortal?.container}>
            <Popover.Content
              side="top"
              align="end"
              sideOffset={6}
              collisionPadding={16}
              aria-label={t("agentToolFullContent")}
              className="z-50 max-h-[min(20rem,var(--radix-popover-content-available-height))] w-max max-w-[min(40rem,calc(100vw-2rem))] overflow-y-auto rounded-md bg-foreground px-3 py-2 text-left font-mono text-xs leading-5 whitespace-pre-wrap break-all text-background shadow-md outline-none"
            >
              {value}
              <Popover.Arrow className="fill-foreground" />
            </Popover.Content>
          </Popover.Portal>
        </Popover.Root>
      ) : null}
    </div>
  )
}

/** 返回工具执行状态文案。 */
function useToolStatusLabel() {
  const { t } = useTranslation("inbox")
  return (status: AgentToolCallStatus) => ({
    [AgentToolCallStatus.AgentToolCallQueued]: t("agentToolQueued"),
    [AgentToolCallStatus.AgentToolCallRunning]: t("agentToolRunning"),
    [AgentToolCallStatus.AgentToolCallSucceeded]: t("agentToolSucceeded"),
    [AgentToolCallStatus.AgentToolCallFailed]: t("agentToolFailed"),
  })[status]
}

/** 展示单次工具调用并按需展开完整参数、结果或错误。 */
function AgentTool({ call }: { call: AgentToolCall }) {
  const { t } = useTranslation("inbox")
  const failed = call.status === AgentToolCallStatus.AgentToolCallFailed
  const statusLabel = useToolStatusLabel()(call.status)
  return (
    <Collapsible className="min-w-0 rounded-md bg-muted text-foreground">
      <CollapsibleTrigger className="group flex w-full min-w-0 items-center gap-3 rounded-md px-3 py-2 text-left text-xs focus-visible:outline focus-visible:outline-ring">
        <span className="min-w-0 flex-1 break-all font-medium">{call.name}</span>
        <span className={cn("shrink-0 text-muted-foreground", failed && "text-destructive")}>
          {statusLabel}
        </span>
        <ChevronDownIcon aria-hidden className="size-3.5 shrink-0 transition-transform group-data-[state=open]:rotate-180" />
      </CollapsibleTrigger>
      <CollapsibleContent className="px-3 pb-3">
        <Tabs defaultValue="arguments">
          <TabsList className="gap-4">
            <TabsTrigger value="arguments" className="pb-2 text-xs">
              {t("agentToolArguments")}
            </TabsTrigger>
            <TabsTrigger value="result" className="pb-2 text-xs">
              {t("agentToolResult")}
            </TabsTrigger>
          </TabsList>
          <TabsContent value="arguments" className="pt-2">
            <ToolValue value={call.arguments} />
          </TabsContent>
          <TabsContent value="result" className="space-y-1 pt-2">
            {call.error !== null ? (
              <p className="text-xs text-destructive">{t("agentToolError")}</p>
            ) : null}
            <ToolValue value={call.error ?? call.result ?? statusLabel} />
          </TabsContent>
        </Tabs>
      </CollapsibleContent>
    </Collapsible>
  )
}

/** 沿头像侧对齐思考标题、右上角显示本次模型用量，首次展开时按运行编号读取过程内容。 */
export function AgentProcess({ process, incoming }: { process: ConversationAgentProcessData; incoming: boolean }) {
  const { t, i18n } = useTranslation(["inbox", "common"])
  const [opened, setOpened] = useState(false)
  const mobile = resolveAppPlatform() === "mobile"
  const seconds = Math.max(0, Math.round(process.durationMilliseconds / 1000))
  // 成功运行的过程内容不可变，首次展开后按运行编号读取并长期复用缓存。
  const detail = useResource(
    resourceKeys.agentRunProcess(process.id),
    (signal) => getAgentRunProcess(process.id, signal),
    { enabled: opened, staleTime: Infinity },
  )
  return (
    <Collapsible className="mb-3 min-w-0" onOpenChange={(open) => open && setOpened(true)}>
      <div className={cn(
        "flex gap-3",
        // 移动端窄屏把用量换到下一行，思考标题保持完整。
        mobile ? "flex-col gap-1" : "items-center",
      )}>
        <CollapsibleTrigger className={cn(
          "group flex min-w-0 flex-1 cursor-pointer items-center gap-1.5 rounded-sm py-1 text-left text-xs focus-visible:outline focus-visible:outline-ring",
          incoming ? "justify-start text-muted-foreground" : "justify-end text-primary-foreground/75",
          // 移动端按触屏点击区域抬高行高并占满气泡宽度，点击区不与引用块和正文重叠。
          mobile && "w-full py-2",
        )}>
          <LightbulbIcon aria-hidden className="size-4 shrink-0 text-yellow-400 dark:text-yellow-300" />
          <span className="truncate">{t("agentThoughtCompleted", { seconds })}</span>
          <ChevronDownIcon aria-hidden className="size-3.5 shrink-0 transition-transform group-data-[state=open]:rotate-180" />
        </CollapsibleTrigger>
        <div className={cn(
          "flex shrink-0 gap-2 text-[11px]",
          incoming ? "text-muted-foreground" : "text-primary-foreground/75",
          mobile && (incoming ? "self-start" : "self-end"),
        )}>
          <span>{t("agentUsageInput", { count: process.inputTokens })}</span>
          <span>{t("agentUsageOutput", { count: process.outputTokens })}</span>
        </div>
      </div>
      <CollapsibleContent className={cn(
        "mt-2 space-y-3 text-left text-sm",
        incoming ? "border-l border-border pl-3" : "border-r border-primary-foreground/30 pr-3",
      )}>
        {detail.data ? detail.data.blocks.map((block) =>
          block.kind === AgentRunBlockKind.AgentRunBlockToolCall && block.toolCall ? (
            <AgentTool key={block.id} call={block.toolCall} />
          ) : (
            <div
              key={block.id}
              className={cn(
                "min-w-0 break-words",
                block.kind === AgentRunBlockKind.AgentRunBlockThinking &&
                  cn("italic", incoming ? "text-muted-foreground" : "text-primary-foreground/75"),
              )}
            >
              <MessageMarkdown locale={i18n.language} onOpenLink={openExternalURL}>{block.text}</MessageMarkdown>
            </div>
          ),
        ) : (
          // 读取中与读取失败共用一行占位，保持展开区域高度稳定。
          <p className={cn("text-xs", incoming ? "text-muted-foreground" : "text-primary-foreground/75")}>
            {detail.error && !detail.refreshing ? (
              <>
                <span>{isApiError(detail.error) ? apiErrorMessage(detail.error) : t("agentProcessLoadFailed")}</span>
                <button
                  type="button"
                  className="ml-2 rounded-sm underline focus-visible:outline focus-visible:outline-ring"
                  onClick={() => void detail.refresh()}
                >
                  {t("common:actions.retry")}
                </button>
              </>
            ) : t("common:status.loading")}
          </p>
        )}
      </CollapsibleContent>
    </Collapsible>
  )
}

/** 展示运行中工具调用的名称与状态，完整参数和结果在运行成功后读取。 */
function AgentStreamTool({ call }: { call: RunStreamToolCall }) {
  const statusLabel = useToolStatusLabel()
  return (
    <div className="flex min-w-0 items-center gap-3 rounded-md bg-muted px-3 py-2 text-xs text-foreground">
      <span className="min-w-0 flex-1 break-all font-medium">{call.name}</span>
      <span className={cn(
        "shrink-0 text-muted-foreground",
        call.status === AgentToolCallStatus.AgentToolCallFailed && "text-destructive",
      )}>
        {statusLabel(call.status)}
      </span>
    </div>
  )
}

/** 按序渲染运行过程流中的思考、工具调用和正在生成的回复正文；区域限高滚动，内容增长时保持贴底。 */
function AgentRunStreamProcess({ state, incoming }: { state: RunStreamState; incoming: boolean }) {
  const { i18n } = useTranslation("inbox")
  const muted = incoming ? "text-muted-foreground" : "text-primary-foreground/75"
  const scroll = useRef<HTMLDivElement>(null)
  const following = useRef(true)
  useLayoutEffect(() => {
    const node = scroll.current
    if (node && following.current) node.scrollTop = node.scrollHeight
  }, [state])
  return (
    <div
      ref={scroll}
      // 限高让状态行与停止按钮始终可见；用户上滚查看早先过程时不再自动贴底。
      className="max-h-64 space-y-3 overflow-y-auto"
      onScroll={(event) => {
        const node = event.currentTarget
        following.current = node.scrollHeight - node.scrollTop - node.clientHeight <= 16
      }}
    >
      {state.blocks.map((block) =>
        block.kind === AgentRunBlockKind.AgentRunBlockToolCall && block.toolCall ? (
          <AgentStreamTool key={block.id} call={block.toolCall} />
        ) : (
          <div
            key={block.id}
            className={cn("min-w-0 break-words", block.kind === AgentRunBlockKind.AgentRunBlockThinking && cn("italic", muted))}
          >
            <MessageMarkdown locale={i18n.language} onOpenLink={openExternalURL}>{block.text}</MessageMarkdown>
          </div>
        ),
      )}
      {state.candidateContent ? (
        <div className="min-w-0 break-words">
          <MessageMarkdown locale={i18n.language} onOpenLink={openExternalURL}>{state.candidateContent}</MessageMarkdown>
        </div>
      ) : null}
    </div>
  )
}

/** 显示一次尚未由消息表达的运行的等待、思考或取消状态，运行中默认展开实时过程，取消运行可展开中断前的过程。 */
export function AgentRunState({ run, incoming, conversationID, group, copilot, onStopped }: { run: ConversationAgentRun; incoming: boolean; conversationID?: string; group?: boolean; copilot?: boolean; onStopped: () => Promise<unknown> }) {
  const { t } = useTranslation("inbox")
  const stream = useAgentRunStream(run.id, run.status === AgentRunStatus.AgentRunStatusRunning, onStopped)
  if (run.status === AgentRunStatus.AgentRunStatusSucceeded || run.status === AgentRunStatus.AgentRunStatusFailed ||
    (run.status === AgentRunStatus.AgentRunStatusCancelled && run.errorCode === "user_cancelled")) return null
  const thinking = run.status === AgentRunStatus.AgentRunStatusRunning
  const cancelled = run.status === AgentRunStatus.AgentRunStatusCancelled
  const senderName = run.agentName.trim() || t("unknownSender")
  const label = thinking
    ? t("agentThoughtRunning")
    : cancelled
      ? t("agentRunCancelled")
      : t("agentRunQueued")
  const reason = run.errorCode === "assignee_changed"
    ? t("agentRunAssigneeChanged")
    : run.errorCode === "session_closed"
      ? t("agentRunSessionClosed")
      : run.errorCode === "bot_changed"
        ? t("agentRunBotChanged")
        : run.errorCode === "agent_removed"
          ? t("agentRunAgentRemoved")
          : run.lastError
  return (
    <div
      className={cn("mt-3 flex min-w-0 text-xs text-muted-foreground", incoming ? "justify-start" : "justify-end")}
      role="status"
      aria-label={`${senderName} ${label}`}
    >
      <div className={cn(
        "relative flex min-h-8 max-w-[75%] flex-col justify-center py-2",
        // 运行中的过程与最终消息气泡同宽，结束后替换为消息时不再重新换行。
        thinking && "max-w-[min(36rem,85%)] sm:max-w-[min(36rem,75%)]",
        incoming ? "ml-10" : "mr-10",
      )}>
        <ProfileAvatar
          imageURL={run.agentAvatarUrl}
          name={run.agentName}
          fallback="agent"
          title={senderName}
          className={cn(
            "absolute bottom-0 size-8 text-xs",
            incoming ? "right-full mr-2" : "left-full ml-2",
          )}
        />
        {group && incoming ? (
          <span className="mb-1 max-w-full truncate text-xs font-medium text-foreground">{senderName}</span>
        ) : null}
        {thinking ? (
          <Collapsible defaultOpen className="min-w-0">
            <div className={cn(
              "flex items-center gap-1.5",
              // 移动端留出停止按钮触屏区域向左溢出的宽度，两个点击区域不重叠。
              resolveAppPlatform() === "mobile" && "gap-3",
            )}>
              <CollapsibleTrigger className={cn(
                "group flex min-w-0 cursor-pointer items-center gap-1.5 rounded-sm text-left focus-visible:outline focus-visible:outline-ring",
                // 移动端按触屏点击区域抬高行高，点击区不与展开的过程内容重叠。
                resolveAppPlatform() === "mobile" && "py-2",
              )}>
                <span className="truncate">{label}</span>
                <ChevronDownIcon aria-hidden className="size-3.5 shrink-0 transition-transform group-data-[state=open]:rotate-180" />
              </CollapsibleTrigger>
              {conversationID ? <AgentReplyStopButton conversationID={conversationID} runID={run.id} group={group} copilot={copilot} onStopped={onStopped} /> : null}
            </div>
            <CollapsibleContent className={cn(
              "text-left text-sm",
              // 首个快照到达前不占位，展开区域不出现空的缩进边框。
              stream && cn("mt-2", incoming ? "border-l border-border pl-3" : "border-r border-primary-foreground/30 pr-3"),
            )}>
              {stream ? <AgentRunStreamProcess state={stream} incoming={incoming} /> : null}
            </CollapsibleContent>
          </Collapsible>
        ) : (
          <>
            {run.process ? <AgentProcess process={run.process} incoming={incoming} /> : null}
            <div className="flex items-center gap-1.5">
              {cancelled ? <BrainIcon aria-hidden className="size-4" /> : null}
              <span>{label}</span>
              {conversationID && !cancelled ? <AgentReplyStopButton conversationID={conversationID} runID={run.id} group={group} copilot={copilot} onStopped={onStopped} /> : null}
            </div>
          </>
        )}
        {cancelled && reason ? (
          <p className="mt-1 whitespace-pre-wrap break-all">{reason}</p>
        ) : null}
      </div>
    </div>
  )
}

/** 展示已收到点名、等待轮转发言的 AI 员工。 */
export function AgentQueueState({ agents, incoming }: { agents: ConversationPendingAgent[]; incoming: boolean }) {
  const { t } = useTranslation("inbox")
  if (!agents.length) return null
  const names = agents.map((agent) => agent.displayName.trim() || t("unknownSender")).join("、")
  return (
    <div
      className={cn("mt-2 flex min-w-0 text-xs text-muted-foreground", incoming ? "justify-start" : "justify-end")}
      role="status"
    >
      <span className={cn("max-w-[75%] break-all", incoming ? "ml-10" : "mr-10")}>
        {t("agentRunWaiting", { names })}
      </span>
    </div>
  )
}

/** 停止指定运行后刷新原会话资源，卸载后忽略交互结果。 */
function AgentReplyStopButton({ conversationID, runID, group, copilot, onStopped }: { conversationID: string; runID: string; group?: boolean; copilot?: boolean; onStopped: () => Promise<unknown> }) {
  const { t } = useTranslation("inbox")
  const navigate = useNavigate()
  const invalidate = useResourceInvalidator()
  const [stopping, setStopping] = useState(false)
  const alive = useRef(true)
  useEffect(() => {
    alive.current = true
    return () => { alive.current = false }
  }, [])

  /** 提交停止命令并通过查询读取最终状态和消息。 */
  async function stop() {
    setStopping(true)
    try {
      // 按会话类型选择 Copilot 线程、群聊或独立 AI 会话的停止入口。
      if (copilot) await stopCustomerCopilotReply(conversationID, runID)
      else if (group) await stopGroupAgentReply(conversationID, runID)
      else await stopAgentReply(conversationID, runID)
      await Promise.all([
        invalidate(resourceKeys.conversationMessages(conversationID)),
        invalidate(resourceKeys.conversationMessagePage(conversationID)),
        invalidate(resourceKeys.inbox()),
      ])
      if (alive.current) await onStopped()
    } catch (error) {
      if (!alive.current || recoverSession(error, navigate)) return
      toast.error(isApiError(error) ? apiErrorMessage(error) : t("agentStopFailed"))
    } finally {
      if (alive.current) setStopping(false)
    }
  }

  return (
    <button
      type="button"
      className={cn(
        "inline-flex size-5 shrink-0 items-center justify-center rounded-sm text-destructive focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring disabled:pointer-events-none disabled:opacity-50",
        // 移动端扩大触屏点击区域，负外边距保持原有行高和图标位置。
        resolveAppPlatform() === "mobile" && "-m-3 size-11",
      )}
      aria-label={t("agentStopReply")}
      title={t("agentStopReply")}
      disabled={stopping}
      onClick={() => void stop()}
    >
      <SquareIcon aria-hidden className="size-3 fill-current" />
    </button>
  )
}
