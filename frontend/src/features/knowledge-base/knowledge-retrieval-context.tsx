/** 在检索侧栏中读取并展示命中分段的上下文。 */
import { useEffect, useRef } from "react"
import { useTranslation } from "react-i18next"

import {
  isApiError,
  readKnowledgeContext,
  type KnowledgeRetrievalResultData,
} from "@/api"
import { SelectableText } from "@/components/selectable-text"
import { Button } from "@/components/ui/button"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"
import { apiErrorMessage } from "@/lib/form-errors"
import { cn } from "@/lib/utils"

/** 展示周边分段、命中标记和返回检索结果的入口。 */
export function KnowledgeRetrievalContext({
  knowledgeBaseId,
  record,
  onBack,
}: {
  knowledgeBaseId: string
  record: KnowledgeRetrievalResultData["records"][number]
  onBack: () => void
}) {
  const { t } = useTranslation("knowledgeBase")
  const backButton = useRef<HTMLButtonElement>(null)
  const { data, loading, error, refresh, refreshing } = useResource(
    resourceKeys.knowledgeContext(
      knowledgeBaseId,
      record.documentId,
      record.segmentId,
      record.position,
    ),
    (signal) =>
      readKnowledgeContext(
        knowledgeBaseId,
        {
          documentId: record.documentId,
          segmentId: record.segmentId,
          position: record.position,
        },
        signal,
      ),
    { staleTime: 0, refetchOnWindowFocus: false },
  )

  // 切换视图后将键盘焦点放到返回入口。
  useEffect(() => {
    backButton.current?.focus()
  }, [])

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div className="flex items-center justify-between gap-3 px-6 py-4">
        <SelectableText className="min-w-0 break-all text-sm font-medium">
          {data?.documentName || record.documentName || record.documentId}
        </SelectableText>
        <Button
          ref={backButton}
          variant="outline"
          size="sm"
          className="shrink-0"
          onClick={onBack}
        >
          {t("retrieval.backToResults")}
        </Button>
      </div>
      <div
        className="min-h-0 flex-1 overflow-y-auto border-t px-6 py-5"
        aria-live="polite"
        aria-busy={loading || refreshing}
      >
        {loading ? (
          <p className="py-12 text-center text-sm text-muted-foreground">
            {t("retrieval.contextLoading")}
          </p>
        ) : error ? (
          <div className="space-y-4 py-12 text-center">
            <p className="text-sm text-destructive">
              {isApiError(error)
                ? apiErrorMessage(error, ["context"])
                : t("retrieval.contextError")}
            </p>
            <Button
              variant="outline"
              size="sm"
              disabled={refreshing}
              onClick={() => void refresh()}
            >
              {t("retry")}
            </Button>
          </div>
        ) : (
          <ol className="divide-y">
            {data?.segments.map((segment) => (
              <li
                key={segment.segmentId}
                className={cn(
                  "py-5",
                  segment.matched && "border-l-2 border-l-primary pl-3",
                )}
              >
                <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
                  <span>
                    {t("retrieval.position", { position: segment.position })}
                  </span>
                  {segment.matched && (
                    <span className="font-medium text-primary">
                      {t("retrieval.matched")}
                    </span>
                  )}
                </div>
                <SelectableText className="mt-3 block whitespace-pre-wrap break-words text-sm leading-6">
                  {segment.content || "—"}
                </SelectableText>
                {segment.answer?.trim() && (
                  <div className="mt-4 border-l-2 pl-3">
                    <p className="mb-1 text-xs font-medium text-muted-foreground">
                      {t("retrieval.answer")}
                    </p>
                    <SelectableText className="block whitespace-pre-wrap break-words text-sm leading-6">
                      {segment.answer}
                    </SelectableText>
                  </div>
                )}
              </li>
            ))}
          </ol>
        )}
      </div>
    </div>
  )
}
