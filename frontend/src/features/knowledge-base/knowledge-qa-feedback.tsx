/** 问答页面共用的读取反馈。 */
import { useTranslation } from "react-i18next"
import { isApiError } from "@/api"
import { LoadingIndicator } from "@/components/loading-indicator"
import { Button } from "@/components/ui/button"
import { apiErrorMessage } from "@/lib/form-errors"

/** 展示问答读取错误及重试入口，或初次读取进度。 */
export function KnowledgeQAFeedback({
  error,
  retry,
}: {
  error: unknown
  retry: () => void
}) {
  const { t } = useTranslation("knowledgeBase")
  if (!error)
    return (
      <LoadingIndicator className="min-h-48 justify-center">
        {t("loading")}
      </LoadingIndicator>
    )
  return (
    <div className="flex min-h-48 flex-col items-center justify-center gap-4">
      <p className="text-sm text-muted-foreground">
        {isApiError(error) ? apiErrorMessage(error) : t("qa.loadError")}
      </p>
      <Button variant="outline" onClick={retry}>
        {t("retry")}
      </Button>
    </div>
  )
}
