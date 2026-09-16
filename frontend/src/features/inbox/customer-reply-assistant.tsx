/** 客户会话输入区的 AI 写回复弹层，桌面端使用 Popover，移动端使用底部 Sheet。 */
import { useEffect, useId, useRef, useState } from "react"
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
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from "@/components/ui/sheet"
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
  mobile = false,
  onApply,
}: {
  conversationID: string
  currentIdentityID: string
  draft: string
  replyToMessageID: string
  disabled: boolean
  mobile?: boolean
  onApply: (reply: string) => void
}) {
  const { t } = useTranslation(["inbox", "common"])
  const fieldPrefix = useId()
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

  /** 打开弹层时按草稿是否为空选中改写或写回复，并以当前草稿和引用作为生成条件。 */
  function changeOpen(nextOpen: boolean) {
    if (nextOpen) {
      setMode(
        draft.trim()
          ? CustomerReplyMode.CustomerReplyModeRewrite
          : CustomerReplyMode.CustomerReplyModeReply,
      )
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

  /** 以候选替换当前对客草稿并关闭弹层。 */
  function applyCandidate(candidate: string) {
    appliedRef.current = true
    onApply(candidate)
    setOpen(false)
    void removeResource(resourceKeys.customerReplySuggestions(conversationID))
  }

  /** 使用候选后焦点交给回复输入框。 */
  function keepAppliedFocus(event: Event) {
    if (!appliedRef.current) return
    appliedRef.current = false
    event.preventDefault()
  }

  const trigger = (
    <Button
      type="button"
      variant="ghost"
      size="icon-sm"
      className={mobile ? "size-11" : undefined}
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
  )

  const modeSelector = (
    <div
      role="radiogroup"
      aria-label={t("replyAssistantMode")}
      className={cn(
        "flex shrink-0 rounded-md border bg-background p-0.5",
        mobile && "w-full",
      )}
    >
      {replyModes.map((value) => (
        <button
          key={value}
          type="button"
          role="radio"
          aria-checked={mode === value}
          className={cn(
            "rounded-sm font-medium whitespace-nowrap transition-colors outline-none focus-visible:ring-2 focus-visible:ring-ring/50",
            mobile ? "h-10 flex-1 text-sm" : "h-7 px-2.5 text-xs",
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
  )

  const agentSelect = (
    <NativeSelect
      id={mobile ? `${fieldPrefix}-agent` : undefined}
      aria-label={mobile ? undefined : t("replyAssistantAgent")}
      className={cn(
        mobile
          ? "min-h-11 w-full text-sm"
          : "h-7 w-auto max-w-36 truncate px-2 pr-8 text-xs shadow-none",
      )}
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
  )

  const toneSelect = (
    <NativeSelect
      id={mobile ? `${fieldPrefix}-tone` : undefined}
      aria-label={mobile ? undefined : t("replyAssistantTone")}
      className={cn(
        mobile ? "min-h-11 w-full text-sm" : "h-7 w-auto px-2 pr-8 text-xs shadow-none",
      )}
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
  )

  const results = (
    <div
      aria-live="polite"
      aria-busy={generating}
      className={cn(
        "overflow-y-auto",
        mobile ? "h-56 min-h-0 px-4" : "h-56 max-h-[calc(100dvh-17rem)] pr-1",
      )}
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
          {candidates.map((candidate, index) =>
            mobile ? (
              <li key={index}>
                <button
                  type="button"
                  className="w-full rounded-md border bg-background p-3 text-left text-sm leading-6 whitespace-pre-wrap outline-none active:bg-muted focus-visible:ring-2 focus-visible:ring-ring/50"
                  onClick={() => applyCandidate(candidate)}
                >
                  {candidate}
                </button>
              </li>
            ) : (
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
                  onClick={() => applyCandidate(candidate)}
                >
                  <PencilLineIcon className="size-4" />
                </Button>
              </li>
            ),
          )}
        </ul>
      ) : (
        <div
          className={cn(
            "flex h-full items-center justify-center rounded-md border border-dashed px-3 text-center text-xs text-muted-foreground",
            mobile && "text-sm",
            (agentOptions.error || (ready && suggestions.error)) && "text-destructive",
          )}
        >
          {emptyMessage}
        </div>
      )}
    </div>
  )

  if (mobile) {
    return (
      <Sheet open={open} onOpenChange={changeOpen}>
        <SheetTrigger asChild>{trigger}</SheetTrigger>
        <SheetContent
          side="bottom"
          showCloseButton={false}
          aria-describedby={undefined}
          className="max-h-[calc(100dvh-env(safe-area-inset-top)-1rem)] gap-0 overflow-hidden rounded-t-2xl pb-[env(safe-area-inset-bottom)]"
          onCloseAutoFocus={keepAppliedFocus}
        >
          <SheetHeader className="shrink-0 flex-row items-center border-b">
            <SheetTitle className="flex-1">{t("replyAssistant")}</SheetTitle>
            <SheetClose asChild>
              <Button variant="ghost" className="min-h-11">
                {t("common:actions.cancel")}
              </Button>
            </SheetClose>
          </SheetHeader>
          <div className="shrink-0 space-y-3 p-4">
            {modeSelector}
            <div className="grid grid-cols-2 gap-2">
              <div className="space-y-1.5">
                <label
                  htmlFor={`${fieldPrefix}-agent`}
                  className="block text-xs font-medium text-muted-foreground"
                >
                  {t("replyAssistantAgent")}
                </label>
                {agentSelect}
              </div>
              <div className="space-y-1.5">
                <label
                  htmlFor={`${fieldPrefix}-tone`}
                  className="block text-xs font-medium text-muted-foreground"
                >
                  {t("replyAssistantTone")}
                </label>
                {toneSelect}
              </div>
            </div>
          </div>
          {results}
          <div className="shrink-0 p-4">
            <Button
              type="button"
              variant="outline"
              className="min-h-11 w-full"
              disabled={!ready || generating}
              onClick={() => void suggestions.refresh()}
            >
              <RefreshCwIcon className="size-4" />
              {t("replyAssistantRegenerate")}
            </Button>
          </div>
        </SheetContent>
      </Sheet>
    )
  }

  return (
    <Popover open={open} onOpenChange={changeOpen}>
      <PopoverTrigger asChild>{trigger}</PopoverTrigger>
      <PopoverContent
        side="top"
        align="start"
        className="w-[min(36rem,calc(100vw-2rem))] space-y-3 p-3"
        onCloseAutoFocus={keepAppliedFocus}
      >
        <div className="flex items-center gap-2">
          <p className="min-w-0 truncate text-sm font-medium">{t("replyAssistant")}</p>
          {modeSelector}
          <div className="ml-auto flex shrink-0 items-center gap-1">
            {agentSelect}
            {toneSelect}
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
        {results}
      </PopoverContent>
    </Popover>
  )
}
