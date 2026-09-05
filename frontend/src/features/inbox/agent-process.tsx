/** 在聊天消息中展示折叠的 Agent 思考过程、工具详情和模型用量。 */
import { useLayoutEffect, useRef, useState } from "react"
import { BrainIcon, ChevronDownIcon, LoaderCircleIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { Popover } from "radix-ui"

import {
  AgentRunBlockKind,
  AgentRunStatus,
  AgentToolCallStatus,
  type AgentToolCall,
  type ConversationAgentProcessData,
  type ConversationAgentRun,
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
                className="rounded-sm text-primary hover:underline focus-visible:outline focus-visible:outline-ring"
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

/** 展示单次工具调用并按需展开完整参数、结果或错误。 */
function AgentTool({ call }: { call: AgentToolCall }) {
  const { t } = useTranslation("inbox")
  const failed = call.status === AgentToolCallStatus.AgentToolCallFailed
  const statusLabel = {
    [AgentToolCallStatus.AgentToolCallQueued]: t("agentToolQueued"),
    [AgentToolCallStatus.AgentToolCallRunning]: t("agentToolRunning"),
    [AgentToolCallStatus.AgentToolCallSucceeded]: t("agentToolSucceeded"),
    [AgentToolCallStatus.AgentToolCallFailed]: t("agentToolFailed"),
  }[call.status]
  return (
    <Collapsible className="min-w-0 rounded-md bg-muted">
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

/** 按服务端块顺序展示一个默认折叠的已完成思考区域。 */
export function AgentProcess({ process }: { process: ConversationAgentProcessData }) {
  const { t } = useTranslation("inbox")
  const seconds = Math.max(0, Math.round(process.durationMilliseconds / 1000))
  return (
    <Collapsible className="mb-3 min-w-0 text-foreground">
      <CollapsibleTrigger className="group flex w-full items-center gap-1.5 rounded-sm py-1 text-left text-xs text-muted-foreground focus-visible:outline focus-visible:outline-ring">
        <BrainIcon aria-hidden className="size-4 shrink-0" />
        <span>{t("agentThoughtCompleted", { seconds })}</span>
        <ChevronDownIcon aria-hidden className="size-3.5 shrink-0 transition-transform group-data-[state=open]:rotate-180" />
      </CollapsibleTrigger>
      <CollapsibleContent className="mt-2 space-y-3 border-l border-border pl-3 text-sm">
        {process.blocks.map((block) =>
          block.kind === AgentRunBlockKind.AgentRunBlockToolCall && block.toolCall ? (
            <AgentTool key={block.id} call={block.toolCall} />
          ) : (
            <div
              key={block.id}
              className={cn(
                "whitespace-pre-wrap break-words",
                block.kind === AgentRunBlockKind.AgentRunBlockThinking &&
                  "italic text-muted-foreground",
              )}
            >
              {block.text}
            </div>
          ),
        )}
      </CollapsibleContent>
    </Collapsible>
  )
}

/** 在最终正文下方显示本次模型输入和输出用量。 */
export function AgentProcessUsage({ process }: { process: ConversationAgentProcessData }) {
  const { t } = useTranslation("inbox")
  return (
    <div className="mt-2 flex gap-4 text-[11px] text-muted-foreground">
      <span>{t("agentUsageInput", { count: process.inputTokens })}</span>
      <span>{t("agentUsageOutput", { count: process.outputTokens })}</span>
    </div>
  )
}

/** 显示最近一次运行的等待、思考或失败状态，终止运行不展示中间内容。 */
export function AgentRunState({ run, incoming }: { run: ConversationAgentRun; incoming: boolean }) {
  const { t } = useTranslation("inbox")
  if (run.status === AgentRunStatus.AgentRunStatusSucceeded) return null
  const thinking = run.status === AgentRunStatus.AgentRunStatusRunning
  const failed = run.status === AgentRunStatus.AgentRunStatusFailed
  const cancelled = run.status === AgentRunStatus.AgentRunStatusCancelled
  const senderName = run.agentName.trim() || t("unknownSender")
  const senderInitial = Array.from(senderName)[0]?.toLocaleUpperCase() ?? "?"
  const label = thinking
    ? t("agentThoughtRunning")
    : failed
      ? t("agentRunFailed")
      : cancelled
        ? t("agentRunCancelled")
        : t("agentRunQueued")
  const reason = run.errorCode === "assignee_changed"
    ? t("agentRunAssigneeChanged")
    : run.errorCode === "session_closed"
      ? t("agentRunSessionClosed")
      : run.lastError
  return (
    <div
      className={cn("mt-3 flex min-w-0 text-xs text-muted-foreground", incoming ? "justify-start" : "justify-end")}
      role="status"
      aria-label={`${senderName} ${label}`}
    >
      <div className={cn("relative flex min-h-8 max-w-[75%] flex-col justify-center py-2", incoming ? "ml-10" : "mr-10")}>
        <span
          className={cn(
            "absolute bottom-0 flex size-8 items-center justify-center rounded-full text-xs font-medium",
            incoming
              ? "right-full mr-2 border bg-background text-foreground"
              : "left-full ml-2 bg-primary text-primary-foreground",
          )}
          title={senderName}
          aria-hidden="true"
        >
          {senderInitial}
        </span>
        <div className={cn("flex items-center gap-1.5", failed && "text-destructive")}>
          {thinking ? (
            <LoaderCircleIcon aria-hidden className="size-4 animate-spin motion-reduce:animate-none" />
          ) : <BrainIcon aria-hidden className="size-4" />}
          <span>{label}</span>
        </div>
        {(failed || cancelled) && reason ? (
          <p className="mt-1 whitespace-pre-wrap break-all">{reason}</p>
        ) : null}
      </div>
    </div>
  )
}
