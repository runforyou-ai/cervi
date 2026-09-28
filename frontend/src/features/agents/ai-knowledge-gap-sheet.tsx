/** 待补知识处理侧栏：展示来源对话，把 AI 起草的问答编辑后加入知识库，或忽略该条目。 */
import { useEffect, useRef } from "react"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"

import {
  getKnowledgeGap,
  KnowledgeBaseCategory,
  KnowledgeGapDraftStatus,
  KnowledgeGapSource,
  KnowledgeGapStatus,
  listKnowledgeBases,
  type KnowledgeBaseData,
  type KnowledgeGapData,
} from "@/api"
import { ResourceContent } from "@/components/resource-content"
import { Button } from "@/components/ui/button"
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
import { useResource, useResourceInvalidator } from "@/hooks/use-resource"

import { useKnowledgeGapDismiss } from "./use-knowledge-gap-actions"
import { KnowledgeGapForm } from "./knowledge-gap-form"
import { ServiceTranscript } from "./service-transcript"

/** 起草期间轮询详情的间隔。 */
const draftPollInterval = 2000

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
  // 复核来源的提问由起草确定，草稿就绪前不突出显示。
  const questionMessageId =
    gap.source === KnowledgeGapSource.KnowledgeGapSourceKnowledgeGap ||
    gap.source === KnowledgeGapSource.KnowledgeGapSourceInsufficientEvidence ||
    gap.draftStatus === KnowledgeGapDraftStatus.KnowledgeGapDraftStatusReady
      ? gap.questionMessageId
      : ""

  return (
    <ServiceTranscript
      conversationId={gap.conversationId}
      messages={gap.messages}
      highlightedMessageId={questionMessageId}
    />
  )
}
