/** 客户会话输入区的 AI 写回复弹层。 */
import { useEffect, useRef, useState } from "react"
import {
  LoaderCircleIcon,
  PencilLineIcon,
  RefreshCwIcon,
  SparklesIcon,
} from "lucide-react"
import { useTranslation } from "react-i18next"

import {
  CustomerReplyMode,
  CustomerReplyTone,
  generateCustomerReplySuggestions,
  isApiError,
  listCustomerReplyAgents,
} from "@/api"
import { Button } from "@/components/ui/button"
import { NativeSelect } from "@/components/ui/native-select"
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource, useResourceRemover } from "@/hooks/use-resource"
import { apiErrorMessage } from "@/lib/form-errors"
import { cn } from "@/lib/utils"

const replySourceDebounceDelay = 600

const replyModes = [
  CustomerReplyMode.CustomerReplyModeReply,
  CustomerReplyMode.CustomerReplyModeRewrite,
] as const

const replyTones = [
  CustomerReplyTone.CustomerReplyToneKeep,
  CustomerReplyTone.CustomerReplyToneProfessional,
  CustomerReplyTone.CustomerReplyToneFriendly,
  CustomerReplyTone.CustomerReplyToneConcise,
] as const

type ReplyAssistantPreferences = {
  tone: CustomerReplyTone
  agentIdentityId: string
}

/** 按会话上下文生成对客回复候选，使用候选后替换当前草稿。 */
export function CustomerReplyAssistant({
  conversationID,
  currentIdentityID,
  draft,
  replyToMessageID,
  disabled,
  onApply,
}: {
  conversationID: string
  currentIdentityID: string
  draft: string
  replyToMessageID: string
  disabled: boolean
  onApply: (reply: string) => void
}) {
  const { t } = useTranslation(["inbox", "common"])
  const removeResource = useResourceRemover()
  const appliedRef = useRef(false)
  const [open, setOpen] = useState(false)
  const [mode, setMode] = useState(CustomerReplyMode.CustomerReplyModeReply)
  const storageKey = `cervi.inbox.replyAssistant.${currentIdentityID}`
  const [preferences, setPreferences] = useState<ReplyAssistantPreferences>(
    () => {
      // 读取本人在当前企业上次选择的语气和 AI 员工，本机存储不可用时使用默认值。
      try {
        const stored: unknown = JSON.parse(localStorage.getItem(storageKey) ?? "null")
        const value = (stored ?? {}) as Partial<Record<keyof ReplyAssistantPreferences, unknown>>
        return {
          tone: replyTones.find((tone) => tone === value.tone) ?? CustomerReplyTone.CustomerReplyToneKeep,
          agentIdentityId: typeof value.agentIdentityId === "string" ? value.agentIdentityId : "",
        }
      } catch {
        return { tone: CustomerReplyTone.CustomerReplyToneKeep, agentIdentityId: "" }
      }
    },
  )
  const [source, setSource] = useState({ draft, replyToMessageID })

  // 不可使用时关闭弹层，恢复可用后保持关闭。
  if (disabled && open) setOpen(false)

  useEffect(() => {
    if (!open) return
    // 弹层打开期间，草稿和引用变化防抖后才成为新的生成条件。
    const timer = window.setTimeout(
      () => setSource({ draft, replyToMessageID }),
      replySourceDebounceDelay,
    )
    return () => window.clearTimeout(timer)
  }, [draft, open, replyToMessageID])

  useEffect(
    () => () =>
      void removeResource(resourceKeys.customerReplySuggestions(conversationID)),
    [conversationID, removeResource],
  )

  const agentOptions = useResource(
    resourceKeys.customerReplyAgents(),
    listCustomerReplyAgents,
    { enabled: open, staleTime: 0 },
  )
  const agents = agentOptions.data ?? []
  const agentIdentityID = agents.some(
    (agent) => agent.identityId === preferences.agentIdentityId,
  )
    ? preferences.agentIdentityId
    : (agents[0]?.identityId ?? "")
  const rewrite = mode === CustomerReplyMode.CustomerReplyModeRewrite
  const parameters = {
    agentIdentityId: agentIdentityID,
    mode,
    tone: preferences.tone,
    draft: rewrite ? source.draft.trim() : "",
    replyToMessageId: source.replyToMessageID,
  }
  const rewriteEmpty = rewrite && draft.trim() === ""
  const available =
    open && !disabled && agentIdentityID !== "" && !rewriteEmpty
  const ready = available && (!rewrite || parameters.draft !== "")
  // 生成条件落后于当前草稿或引用时，旧结果不再展示，按生成中处理。
  const sourceCurrent =
    source.replyToMessageID === replyToMessageID &&
    (!rewrite || source.draft === draft)
  const suggestions = useResource(
    resourceKeys.customerReplySuggestions(conversationID, parameters),
    () => generateCustomerReplySuggestions(conversationID, parameters),
    { enabled: ready, staleTime: Infinity, refetchOnWindowFocus: false },
  )
  const generating =
    agentOptions.loading ||
    (available &&
      (!sourceCurrent || suggestions.loading || suggestions.refreshing))
  const candidates = suggestions.data?.candidates ?? []
  const modeLabels = {
    [CustomerReplyMode.CustomerReplyModeReply]: t("replyAssistantModeReply"),
    [CustomerReplyMode.CustomerReplyModeRewrite]: t("replyAssistantModeRewrite"),
  }
  const toneLabels = {
    [CustomerReplyTone.CustomerReplyToneKeep]: t("replyAssistantToneKeep"),
    [CustomerReplyTone.CustomerReplyToneProfessional]: t("replyAssistantToneProfessional"),
    [CustomerReplyTone.CustomerReplyToneFriendly]: t("replyAssistantToneFriendly"),
    [CustomerReplyTone.CustomerReplyToneConcise]: t("replyAssistantToneConcise"),
  }
  // 没有候选时的提示按原因排序：员工读取、可用员工、改写草稿和生成失败。
  const emptyMessage = agentOptions.error
    ? t("replyAssistantAgentsLoadError")
    : agents.length === 0
      ? t("agentPickerEmpty")
      : rewriteEmpty
        ? t("replyAssistantRewriteEmpty")
        : suggestions.error
          ? isApiError(suggestions.error)
            ? apiErrorMessage(suggestions.error)
            : t("replyAssistantError")
          : t("replyAssistantEmpty")

  /** 打开弹层时回到写回复模式，并以当前草稿和引用作为生成条件。 */
  function changeOpen(nextOpen: boolean) {
    if (nextOpen) {
      setMode(CustomerReplyMode.CustomerReplyModeReply)
      setSource({ draft, replyToMessageID })
    }
    setOpen(nextOpen)
  }

  /** 更新语气或 AI 员工选择，并在本机保存。 */
  function updatePreferences(next: Partial<ReplyAssistantPreferences>) {
    const merged = { ...preferences, ...next }
    setPreferences(merged)
    try {
      localStorage.setItem(storageKey, JSON.stringify(merged))
    } catch {
      // 本机存储不可用时只在当前页面保留选择。
    }
  }

  return (
    <Popover open={open} onOpenChange={changeOpen}>
      <PopoverTrigger asChild>
        <Button
          type="button"
          variant="ghost"
          size="icon-sm"
          disabled={disabled}
          aria-label={t("replyAssistant")}
          title={t("replyAssistant")}
        >
          {open && generating ? (
            <LoaderCircleIcon className="animate-spin" />
          ) : (
            <SparklesIcon />
          )}
        </Button>
      </PopoverTrigger>
      <PopoverContent
        side="top"
        align="start"
        className="w-[min(36rem,calc(100vw-2rem))] space-y-3 p-3"
        onCloseAutoFocus={(event) => {
          // 使用候选后焦点交给回复输入框。
          if (!appliedRef.current) return
          appliedRef.current = false
          event.preventDefault()
        }}
      >
        <div className="flex items-center gap-2">
          <p className="min-w-0 truncate text-sm font-medium">{t("replyAssistant")}</p>
          <div
            role="radiogroup"
            aria-label={t("replyAssistantMode")}
            className="flex shrink-0 rounded-md border bg-background p-0.5"
          >
            {replyModes.map((value) => (
              <button
                key={value}
                type="button"
                role="radio"
                aria-checked={mode === value}
                className={cn(
                  "h-7 rounded-sm px-2.5 text-xs font-medium whitespace-nowrap transition-colors outline-none focus-visible:ring-2 focus-visible:ring-ring/50",
                  mode === value
                    ? "bg-foreground text-background"
                    : "text-muted-foreground hover:bg-muted hover:text-foreground",
                )}
                onClick={() => setMode(value)}
              >
                {modeLabels[value]}
              </button>
            ))}
          </div>
          <div className="ml-auto flex shrink-0 items-center gap-1">
            <NativeSelect
              aria-label={t("replyAssistantAgent")}
              className="h-7 w-auto max-w-36 truncate px-2 pr-8 text-xs shadow-none"
              value={agentIdentityID}
              disabled={agents.length === 0}
              onChange={(event) =>
                updatePreferences({ agentIdentityId: event.target.value })
              }
            >
              {agents.map((agent) => (
                <option key={agent.identityId} value={agent.identityId}>
                  {agent.displayName}
                </option>
              ))}
            </NativeSelect>
            <NativeSelect
              aria-label={t("replyAssistantTone")}
              className="h-7 w-auto px-2 pr-8 text-xs shadow-none"
              value={preferences.tone}
              onChange={(event) =>
                updatePreferences({ tone: event.target.value as CustomerReplyTone })
              }
            >
              {replyTones.map((tone) => (
                <option key={tone} value={tone}>
                  {toneLabels[tone]}
                </option>
              ))}
            </NativeSelect>
            <Button
              type="button"
              variant="ghost"
              size="icon-sm"
              className="size-7 text-muted-foreground hover:text-foreground"
              disabled={!ready || generating}
              aria-label={t("replyAssistantRegenerate")}
              title={t("replyAssistantRegenerate")}
              onClick={() => void suggestions.refresh()}
            >
              <RefreshCwIcon className="size-4" />
            </Button>
          </div>
        </div>
        <div
          aria-live="polite"
          aria-busy={generating}
          className="h-56 max-h-[calc(100dvh-17rem)] overflow-y-auto pr-1"
        >
          {generating ? (
            <div className="flex h-full items-center justify-center">
              <span className="sr-only">{t("replyAssistantGenerating")}</span>
              <div aria-hidden="true" className="w-28 space-y-2">
                <div className="h-2.5 w-full animate-pulse rounded bg-muted-foreground/25" />
                <div className="h-2.5 w-5/6 animate-pulse rounded bg-muted-foreground/20" />
                <div className="h-2.5 w-2/3 animate-pulse rounded bg-muted-foreground/15" />
              </div>
            </div>
          ) : ready && !suggestions.error && candidates.length > 0 ? (
            <ul aria-label={t("replyAssistantCandidates")} className="space-y-2">
              {candidates.map((candidate, index) => (
                <li
                  key={index}
                  className="flex gap-2 rounded-md border bg-background p-2.5 text-sm leading-6"
                >
                  <p className="min-w-0 flex-1 whitespace-pre-wrap">{candidate}</p>
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon-sm"
                    className="size-7 shrink-0 text-muted-foreground hover:text-foreground"
                    aria-label={t("replyAssistantApply")}
                    title={t("replyAssistantApply")}
                    onClick={() => {
                      appliedRef.current = true
                      onApply(candidate)
                      setOpen(false)
                      void removeResource(resourceKeys.customerReplySuggestions(conversationID))
                    }}
                  >
                    <PencilLineIcon className="size-4" />
                  </Button>
                </li>
              ))}
            </ul>
          ) : (
            <div
              className={cn(
                "flex h-full items-center justify-center rounded-md border border-dashed px-3 text-center text-xs text-muted-foreground",
                (agentOptions.error || (ready && suggestions.error)) && "text-destructive",
              )}
            >
              {emptyMessage}
            </div>
          )}
        </div>
      </PopoverContent>
    </Popover>
  )
}
