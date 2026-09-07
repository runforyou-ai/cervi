/** 展示知识检索命中分段和上下文入口。 */
import { useMemo } from "react"
import { useTranslation } from "react-i18next"

import { type KnowledgeRetrievalResultData } from "@/api"
import { SelectableText } from "@/components/selectable-text"
import { Button } from "@/components/ui/button"

/** 按检索顺序展示命中内容和匹配度。 */
export function KnowledgeRetrievalResults({
  records,
  onViewContext,
}: {
  records: KnowledgeRetrievalResultData["records"]
  onViewContext: (
    record: KnowledgeRetrievalResultData["records"][number],
    trigger: HTMLButtonElement,
  ) => void
}) {
  const { t, i18n } = useTranslation("knowledgeBase")
  const scoreFormatter = useMemo(
    () =>
      new Intl.NumberFormat(i18n.resolvedLanguage, {
        maximumFractionDigits: 3,
      }),
    [i18n.resolvedLanguage],
  )
  return (
    <div>
      <p className="mb-1 text-sm text-muted-foreground">
        {t("retrieval.resultCount", { count: records.length })}
      </p>
      <ol className="divide-y">
        {records.map((record, index) => (
          <li key={`${record.segmentId}-${index}`} className="py-5">
            <div className="flex items-start gap-3">
              <span className="w-5 shrink-0 text-right text-sm font-medium tabular-nums">
                {index + 1}
              </span>
              <div className="min-w-0 flex-1">
                <div className="flex flex-wrap items-baseline gap-x-2 gap-y-1">
                  <SelectableText className="break-all text-sm font-medium">
                    {record.documentName || record.documentId}
                  </SelectableText>
                  <span className="inline-flex items-baseline gap-2 text-xs text-muted-foreground">
                    <span>
                      {t("retrieval.position", {
                        position: record.position,
                      })}
                    </span>
                    <Button
                      type="button"
                      variant="link"
                      className="h-auto p-0 text-xs font-normal"
                      onClick={(event) =>
                        onViewContext(record, event.currentTarget)
                      }
                    >
                      {t("retrieval.viewContext")}
                    </Button>
                  </span>
                </div>
                <SelectableText className="mt-3 block whitespace-pre-wrap break-words text-sm leading-6">
                  {record.content || "—"}
                </SelectableText>
                {record.answer?.trim() ? (
                  <div className="mt-4 border-l-2 pl-3">
                    <p className="mb-1 text-xs font-medium text-muted-foreground">
                      {t("retrieval.answer")}
                    </p>
                    <SelectableText className="block whitespace-pre-wrap break-words text-sm leading-6">
                      {record.answer}
                    </SelectableText>
                  </div>
                ) : null}
              </div>
              <span className="shrink-0 text-xs text-muted-foreground tabular-nums">
                {record.score == null
                  ? t("retrieval.scoreUnavailable")
                  : t("retrieval.score", {
                      score: scoreFormatter.format(record.score),
                    })}
              </span>
            </div>
          </li>
        ))}
      </ol>
    </div>
  )
}
