/** 在聊天消息中展示折叠的 Agent 思考过程、工具详情和模型用量。 */
import { useLayoutEffect, useRef, useState } from "react"
import { BrainIcon, ChevronDownIcon, LoaderCircleIcon } from "lucide-react"
import { useTranslation } from "react-i18next"

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
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip"
import { cn } from "@/lib/utils"

/** 按实际溢出情况为截断的完整原文提供悬停和键盘提示。 */
function ToolValue({ label, value }: { label: string; value: string }) {
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
    <div className="min-w-0 space-y-1">
      <div className="text-xs text-muted-foreground">{label}</div>
      <Tooltip>
        <TooltipTrigger asChild>
          <pre
            ref={element}
            tabIndex={truncated ? 0 : undefined}
            className={cn(
              "line-clamp-4 whitespace-pre-wrap break-all font-mono text-xs leading-5",
              truncated && "cursor-help focus-visible:outline focus-visible:outline-ring",
            )}
          >
            {value}
          </pre>
        </TooltipTrigger>
        {truncated ? (
          <TooltipContent
            side="top"
            className="max-h-80 max-w-[min(40rem,calc(100vw-2rem))] overflow-y-auto whitespace-pre-wrap break-all text-left font-mono text-xs leading-5"
          >
            {value}
          </TooltipContent>
        ) : null}
      </Tooltip>
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
    <Collapsible className="min-w-0 rounded-md bg-muted/70">
      <CollapsibleTrigger className="group flex w-full min-w-0 items-center gap-3 rounded-md px-3 py-2 text-left text-xs focus-visible:outline focus-visible:outline-ring">
        <span className="min-w-0 flex-1 break-all font-medium">{call.name}</span>
        <span className={cn("shrink-0 text-muted-foreground", failed && "text-destructive")}>
          {statusLabel}
        </span>
        <ChevronDownIcon aria-hidden className="size-3.5 shrink-0 transition-transform group-data-[state=open]:rotate-180" />
      </CollapsibleTrigger>
      <CollapsibleContent className="space-y-3 px-3 pb-3">
        <ToolValue label={t("agentToolArguments")} value={call.arguments} />
        {call.result !== null ? (
          <ToolValue label={t("agentToolResult")} value={call.result} />
        ) : null}
        {call.error !== null ? (
          <ToolValue label={t("agentToolError")} value={call.error} />
        ) : null}
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
export function AgentRunState({ run }: { run: ConversationAgentRun }) {
  const { t } = useTranslation("inbox")
  if (run.status === AgentRunStatus.AgentRunStatusSucceeded) return null
  const thinking = run.status === AgentRunStatus.AgentRunStatusRunning
  const failed = run.status === AgentRunStatus.AgentRunStatusFailed
  const cancelled = run.status === AgentRunStatus.AgentRunStatusCancelled
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
    <div className="mx-10 mt-3 min-w-0 text-xs text-muted-foreground" role="status">
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
  )
}
