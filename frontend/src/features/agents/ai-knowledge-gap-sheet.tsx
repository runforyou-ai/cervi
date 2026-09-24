/** 待补知识处理侧栏：展示来源对话，把 AI 起草的问答编辑后加入知识库，或忽略该条目。 */
import { useEffect, useId, useMemo, useRef, useState, type ReactNode } from "react"
import { zodResolver } from "@hookform/resolvers/zod"
import { useForm, type UseFormReturn } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import {
  acceptKnowledgeGap,
  dismissKnowledgeGap,
  getKnowledgeGap,
  getKnowledgeQAEntry,
  isApiError,
  KnowledgeBaseCategory,
  KnowledgeGapDraftStatus,
  KnowledgeGapMessageSender,
  KnowledgeGapSource,
  KnowledgeGapStatus,
  listKnowledgeBases,
  retrieveKnowledgeBase,
  sessionPath,
  type KnowledgeBaseData,
  type KnowledgeGapData,
} from "@/api"
import {
  createQASchema,
  QAFormFields,
  type QAFormValues,
} from "@/components/knowledge-qa-fields"
import { ResourceContent } from "@/components/resource-content"
import { Button } from "@/components/ui/button"
import { Field, FieldLabel } from "@/components/ui/field"
import { NativeSelect } from "@/components/ui/native-select"
import { ScrollArea } from "@/components/ui/scroll-area"
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"
import { useUnsavedChangesContext } from "@/contexts/unsaved-changes-context"
import { resourceKeys } from "@/hooks/resource-keys"
import { useDateTime } from "@/hooks/use-date-time"
import { useFormSave } from "@/hooks/use-form-save"
import {
  useResource,
  useResourceInvalidator,
  useResourceReader,
} from "@/hooks/use-resource"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"
import { cn } from "@/lib/utils"

/** 知识库召回内容的最大字符数。 */
const similarQueryMaxLength = 250

/** 起草期间轮询详情的间隔。 */
const draftPollInterval = 2000

/** 并入的已有问答编号与标准问题。 */
type MergeTarget = { entryId: string; question: string }

/** 按条目编号打开侧栏；gapId 为空时关闭，处理完成后由 onHandled 决定下一条。 */
export function AIKnowledgeGapSheet({
  gapId,
  onClose,
  onHandled,
}: {
  gapId: string
  onClose: () => void
  onHandled: () => void
}) {
  const { t } = useTranslation("agents")
  const { formatDateTime } = useDateTime()
  const invalidate = useResourceInvalidator()
  const unsavedChanges = useUnsavedChangesContext()
  const gap = useResource(
    resourceKeys.knowledgeGap(gapId),
    (signal) => getKnowledgeGap(gapId, signal),
    {
      enabled: Boolean(gapId),
      staleTime: 0,
      // 待处理条目起草期间持续读取，草稿就绪后停止。
      refetchInterval: (data) =>
        data?.status === KnowledgeGapStatus.KnowledgeGapStatusPending &&
        data.draftStatus === KnowledgeGapDraftStatus.KnowledgeGapDraftStatusPending
          ? draftPollInterval
          : false,
    },
  )
  const bases = useResource(resourceKeys.knowledgeBases(), () => listKnowledgeBases(), {
    enabled: Boolean(gapId),
    staleTime: 0,
  })
  const data = gap.data?.id === gapId ? gap.data : undefined
  const draftStatus = useRef(data?.draftStatus)
  useEffect(() => {
    // 起草结束后刷新清单上的草稿标记。
    if (
      draftStatus.current === KnowledgeGapDraftStatus.KnowledgeGapDraftStatusPending &&
      data?.draftStatus !== KnowledgeGapDraftStatus.KnowledgeGapDraftStatusPending
    ) {
      void invalidate(resourceKeys.knowledgeGaps())
    }
    draftStatus.current = data?.draftStatus
  }, [data?.draftStatus, invalidate])

  return (
    <Sheet
      open={Boolean(gapId)}
      onOpenChange={async (open) => {
        // 关闭侧栏会丢弃表单，有未保存内容时先确认。
        if (open || (unsavedChanges && !(await unsavedChanges.confirmDiscard()))) return
        onClose()
      }}
    >
      <SheetContent className="w-full gap-0 p-0 sm:max-w-xl">
        <SheetHeader className="border-b px-6 py-4 pr-12">
          <SheetTitle>{t("performance.gapSheet.title")}</SheetTitle>
          <SheetDescription>
            {data
              ? [
                  t(`performance.gapSources.${data.source}`),
                  data.categoryName || t("performance.uncategorized"),
                  t(`performance.gapTimes.${data.source}`, {
                    time: formatDateTime(data.occurredAt),
                  }),
                ].join(" · ")
              : null}
          </SheetDescription>
        </SheetHeader>
        <ScrollArea className="min-h-0 flex-1">
          <div className="p-6">
            <ResourceContent
              resources={[gap, bases]}
              errorMessage={t("performance.gapSheet.loadError")}
            >
              {data && bases.data ? (
                <KnowledgeGapDetail
                  key={data.id}
                  gap={data}
                  bases={bases.data.knowledgeBases}
                  onHandled={onHandled}
                />
              ) : null}
            </ResourceContent>
          </div>
        </ScrollArea>
      </SheetContent>
    </Sheet>
  )
}

/** 展示来源对话；已加入知识库的条目只展示结果，没有问答知识库时只能忽略，其余展示问答表单。 */
function KnowledgeGapDetail({
  gap,
  bases,
  onHandled,
}: {
  gap: KnowledgeGapData
  bases: KnowledgeBaseData[]
  onHandled: () => void
}) {
  const { t } = useTranslation("agents")
  const navigate = useNavigate()
  const { dismiss, dismissing } = useKnowledgeGapDismiss(gap, onHandled)
  const qaBases = bases.filter(
    (item) => item.category === KnowledgeBaseCategory.KnowledgeBaseCategoryQA,
  )
  const base = bases.find((item) => item.id === gap.knowledgeBaseId)
  const dismissButton =
    gap.status === KnowledgeGapStatus.KnowledgeGapStatusPending ? (
      <Button type="button" variant="outline" disabled={dismissing} onClick={() => void dismiss()}>
        {t("performance.dismissGap")}
      </Button>
    ) : null

  return (
    <div className="space-y-9">
      <KnowledgeGapConversation gap={gap} />
      {gap.status === KnowledgeGapStatus.KnowledgeGapStatusAccepted ? (
        <div className="flex items-center justify-between gap-3 text-sm">
          <p className="min-w-0 truncate text-muted-foreground">
            {base
              ? `${t("performance.gapSheet.accepted")} · ${base.name}`
              : t("performance.gapSheet.accepted")}
          </p>
          {base && gap.qaEntryId ? (
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={() => navigate(`/knowledge-bases/${base.id}/qa/${gap.qaEntryId}/edit`)}
            >
              {t("performance.gapSheet.viewEntry")}
            </Button>
          ) : null}
        </div>
      ) : qaBases.length === 0 ? (
        <div className="space-y-9">
          <p className="text-sm text-muted-foreground">{t("performance.gapSheet.noKnowledgeBase")}</p>
          {dismissButton ? <div className="flex justify-end">{dismissButton}</div> : null}
        </div>
      ) : (
        <KnowledgeGapForm
          gap={gap}
          bases={qaBases}
          dismissButton={dismissButton}
          dismissing={dismissing}
          onHandled={onHandled}
        />
      )}
    </div>
  )
}

/** 列出来源周期的对客沟通，客户提问确定后突出显示。 */
function KnowledgeGapConversation({ gap }: { gap: KnowledgeGapData }) {
  const { t } = useTranslation("agents")
  const navigate = useNavigate()
  // 复核来源的提问由起草确定，草稿就绪前不突出显示。
  const questionMessageId =
    gap.source === KnowledgeGapSource.KnowledgeGapSourceKnowledgeGap ||
    gap.source === KnowledgeGapSource.KnowledgeGapSourceInsufficientEvidence ||
    gap.draftStatus === KnowledgeGapDraftStatus.KnowledgeGapDraftStatusReady
      ? gap.questionMessageId
      : ""

  return (
    <section className="space-y-3">
      <div className="flex items-center justify-between gap-3">
        <h3 className="text-sm font-medium">{t("performance.gapSheet.conversation")}</h3>
        <Button
          type="button"
          variant="link"
          size="sm"
          className="h-auto px-0"
          onClick={() => {
            // 在收件箱打开该会话并定位到客户提问。
            const params = new URLSearchParams({ conversation: gap.conversationId })
            if (questionMessageId) params.set("message", questionMessageId)
            navigate(`/inbox?${params.toString()}`)
          }}
        >
          {t("performance.gapSheet.openConversation")}
        </Button>
      </div>
      <div className="max-h-72 space-y-3 overflow-y-auto rounded-lg border p-3 text-sm">
        {gap.messages.map((message) => (
          <div
            key={message.id}
            className={cn(
              "space-y-0.5 rounded-md px-2 py-1",
              questionMessageId && message.id === questionMessageId && "bg-muted",
            )}
          >
            <p className="text-xs text-muted-foreground">
              {message.sender === KnowledgeGapMessageSender.KnowledgeGapMessageSenderCustomer
                ? t("performance.gapSheet.customer")
                : message.senderName}
            </p>
            <p className="break-words whitespace-pre-wrap">{message.body}</p>
          </div>
        ))}
      </div>
    </section>
  )
}

/** 由条目生成表单值：草稿就绪时取 AI 草稿，否则取客户提问原文。 */
function draftValues(gap: KnowledgeGapData): QAFormValues {
  return {
    question: gap.draft?.question || gap.question,
    similarQuestions: (gap.draft?.similarQuestions ?? []).map((content) => ({ id: "", content })),
    answer: gap.draft?.answer ?? "",
  }
}

/** 编辑问答并加入知识库：草稿变化时同步未改动的表单，可并入知识库中召回的相似问答。 */
function KnowledgeGapForm({
  gap,
  bases,
  dismissButton,
  dismissing,
  onHandled,
}: {
  gap: KnowledgeGapData
  bases: KnowledgeBaseData[]
  dismissButton: ReactNode
  dismissing: boolean
  onHandled: () => void
}) {
  const { t } = useTranslation(["agents", "knowledgeBase"])
  const id = useId()
  const refresh = useKnowledgeGapRefresh()
  const schema = useMemo(
    () =>
      createQASchema({
        question: t("knowledgeBase:qa.questionRequired"),
        answer: t("knowledgeBase:qa.answerRequired"),
      }),
    [t],
  )
  const form = useForm<QAFormValues>({
    resolver: zodResolver(schema),
    shouldUseNativeValidation: true,
    defaultValues: draftValues(gap),
  })
  const [knowledgeBaseId, setKnowledgeBaseId] = useState(
    bases.some((item) => item.id === gap.defaultKnowledgeBaseId)
      ? gap.defaultKnowledgeBaseId
      : (bases[0]?.id ?? ""),
  )
  const [merge, setMerge] = useState<MergeTarget | null>(null)
  const [draftArrived, setDraftArrived] = useState(false)
  const merging = useMergeInto(form, gap, knowledgeBaseId, setMerge)
  const { submit } = useFormSave({
    form,
    schema,
    autoSave: false,
    save: (values) =>
      acceptKnowledgeGap(gap.id, { knowledgeBaseId, entryId: merge?.entryId ?? "", entry: values }),
    onSubmitted: () => {
      toast.success(t("performance.gapSheet.acceptSuccess"))
      onHandled()
      refresh(gap.id, knowledgeBaseId)
    },
    errorMessage: t("performance.gapSheet.acceptError"),
    errorFields: ["question", "similarQuestions", "answer"],
    logLabel: "加入知识库",
  })
  const appliedDraft = useRef(JSON.stringify(gap.draft))
  useEffect(() => {
    // 草稿变化时同步未改动的表单；表单已改动或已并入时，新草稿到达后提示替换。
    const current = JSON.stringify(gap.draft)
    if (current === appliedDraft.current) return
    appliedDraft.current = current
    if (!form.formState.isDirty && !merge) {
      form.reset(draftValues(gap))
      setDraftArrived(false)
    } else if (gap.draft) {
      setDraftArrived(true)
    }
  }, [gap, form, merge])
  const query = (gap.draft?.question || gap.question).trim().slice(0, similarQueryMaxLength)
  const similar = useResource(
    resourceKeys.knowledgeGapSimilarQA(gap.id, knowledgeBaseId, query),
    () => retrieveKnowledgeBase(knowledgeBaseId, { query }),
    { enabled: Boolean(knowledgeBaseId && query) },
  )
  const busy = form.formState.isSubmitting || dismissing || merging.loading

  return (
    <form className="space-y-9" onSubmit={form.handleSubmit(submit)} noValidate>
      <div className="space-y-6">
        <KnowledgeGapDraftNotice
          gap={gap}
          draftArrived={draftArrived}
          disabled={busy}
          onUseDraft={() => {
            form.reset(draftValues(gap))
            setMerge(null)
            setDraftArrived(false)
          }}
        />
        <Field>
          <FieldLabel htmlFor={`${id}-knowledge-base`} required>
            {t("performance.gapSheet.knowledgeBase")}
          </FieldLabel>
          <NativeSelect
            id={`${id}-knowledge-base`}
            value={knowledgeBaseId}
            required
            disabled={busy || merge !== null}
            onChange={(event) => setKnowledgeBaseId(event.target.value)}
          >
            {bases.map((item) => (
              <option key={item.id} value={item.id}>
                {item.name}
              </option>
            ))}
          </NativeSelect>
        </Field>
        <KnowledgeGapMergeHint
          merge={merge}
          match={similar.data?.records[0]?.content ?? ""}
          disabled={busy}
          onMerge={() => {
            const match = similar.data?.records[0]
            if (match) void merging.mergeInto(match.documentId)
          }}
          onCreateNew={() => {
            form.reset(draftValues(gap))
            setMerge(null)
          }}
        />
        <QAFormFields control={form.control} disabled={busy} answerRows={8} />
      </div>
      <div className="flex justify-end gap-2">
        {dismissButton}
        <Button type="submit" disabled={busy}>
          {form.formState.isSubmitting
            ? t("performance.gapSheet.accepting")
            : t("performance.gapSheet.accept")}
        </Button>
      </div>
    </form>
  )
}

/** 说明 AI 起草的进度：起草中、失败、未设置小结模型，或草稿在表单改动后到达。 */
function KnowledgeGapDraftNotice({
  gap,
  draftArrived,
  disabled,
  onUseDraft,
}: {
  gap: KnowledgeGapData
  draftArrived: boolean
  disabled: boolean
  onUseDraft: () => void
}) {
  const { t } = useTranslation("agents")
  if (draftArrived)
    return (
      <p className="flex flex-wrap items-center gap-x-2 text-sm text-muted-foreground">
        {t("performance.gapSheet.draftArrived")}
        <Button type="button" variant="link" size="sm" className="h-auto px-0" disabled={disabled} onClick={onUseDraft}>
          {t("performance.gapSheet.useDraft")}
        </Button>
      </p>
    )
  const notice = {
    [KnowledgeGapDraftStatus.KnowledgeGapDraftStatusPending]: t("performance.gapSheet.drafting"),
    [KnowledgeGapDraftStatus.KnowledgeGapDraftStatusFailed]: t("performance.gapSheet.draftFailed"),
    [KnowledgeGapDraftStatus.KnowledgeGapDraftStatusUnavailable]: t("performance.gapSheet.draftUnavailable"),
  }[gap.draftStatus as string]
  return notice ? <p className="text-sm text-muted-foreground">{notice}</p> : null
}

/** 并入已有问答时提示将更新的问答并可改为新建，否则在召回到相似问答时提示并入。 */
function KnowledgeGapMergeHint({
  merge,
  match,
  disabled,
  onMerge,
  onCreateNew,
}: {
  merge: MergeTarget | null
  match: string
  disabled: boolean
  onMerge: () => void
  onCreateNew: () => void
}) {
  const { t } = useTranslation("agents")
  if (!merge && !match) return null
  return (
    <p className="flex flex-wrap items-center gap-x-2 text-sm text-muted-foreground">
      {merge
        ? t("performance.gapSheet.merging", { question: merge.question })
        : t("performance.gapSheet.similar", { question: match })}
      <Button
        type="button"
        variant="link"
        size="sm"
        className="h-auto px-0"
        disabled={disabled}
        onClick={merge ? onCreateNew : onMerge}
      >
        {t(merge ? "performance.gapSheet.createNew" : "performance.gapSheet.merge")}
      </Button>
    </p>
  )
}

/** 读取召回的已有问答并填入表单，把本条问题追加为相似问题；读取期间知识库已切换时丢弃结果。 */
function useMergeInto(
  form: UseFormReturn<QAFormValues>,
  gap: KnowledgeGapData,
  knowledgeBaseId: string,
  onMerged: (target: MergeTarget) => void,
) {
  const { t } = useTranslation("agents")
  const read = useResourceReader()
  const [loading, setLoading] = useState(false)
  const currentBase = useRef(knowledgeBaseId)
  currentBase.current = knowledgeBaseId

  /** 并入指定问答。 */
  async function mergeInto(entryId: string) {
    const requestedBase = knowledgeBaseId
    setLoading(true)
    try {
      const entry = await read(resourceKeys.knowledgeQAEntry(requestedBase, entryId), (signal) =>
        getKnowledgeQAEntry(requestedBase, entryId, signal),
      )
      if (currentBase.current !== requestedBase) return
      const values = form.getValues()
      const existing = new Set([entry.question, ...entry.similarQuestions.map((item) => item.content)])
      const added = [values.question, ...values.similarQuestions.map((item) => item.content)]
        .map((content) => content.trim())
        .filter((content) => content && !existing.has(content))
      form.reset({
        question: entry.question,
        similarQuestions: [...entry.similarQuestions, ...added.map((content) => ({ id: "", content }))],
        answer: entry.answer,
      })
      onMerged({ entryId: entry.id, question: entry.question })
    } catch (error) {
      if (isApiError(error) && sessionPath(error.state)) return
      console.warn("读取已有问答失败", { gapId: gap.id, error })
      toast.error(t("performance.gapSheet.mergeError"))
    } finally {
      setLoading(false)
    }
  }

  return { loading, mergeInto }
}

/** 返回刷新处理结果影响的清单、详情、报表与知识库问答的函数。 */
function useKnowledgeGapRefresh() {
  const invalidate = useResourceInvalidator()
  return (gapId: string, knowledgeBaseId?: string) => {
    void Promise.all([
      invalidate(resourceKeys.knowledgeGaps()),
      invalidate(resourceKeys.knowledgeGap(gapId)),
      invalidate(resourceKeys.aiPerformanceReport()),
      knowledgeBaseId ? invalidate(resourceKeys.knowledgeQAEntries(knowledgeBaseId)) : undefined,
    ])
  }
}

/** 提供忽略操作：成功后先交给下一条，再刷新相关读取。 */
function useKnowledgeGapDismiss(gap: KnowledgeGapData, onHandled: () => void) {
  const { t } = useTranslation("agents")
  const navigate = useNavigate()
  const refresh = useKnowledgeGapRefresh()
  const [dismissing, setDismissing] = useState(false)

  /** 忽略该条目。 */
  async function dismiss() {
    setDismissing(true)
    try {
      await dismissKnowledgeGap(gap.id)
      onHandled()
      refresh(gap.id)
    } catch (error) {
      setDismissing(false)
      if (recoverSession(error, navigate)) return
      console.warn("忽略待补知识失败", error)
      toast.error(isApiError(error) ? apiErrorMessage(error) : t("performance.gapSheet.dismissError"))
    }
  }

  return { dismiss, dismissing }
}
