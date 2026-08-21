/** 会话加载失败状态。 */
import { useTranslation } from "react-i18next"

import { Button } from "@/components/ui/button"

/** 展示会话加载失败提示。 */
export function SessionLoadFailedState({ onRetry }: { onRetry: () => void }) {
  const { t } = useTranslation("common")

  return (
    <main className="flex min-h-dvh items-center justify-center p-6">
      <div className="max-w-sm text-center">
        <p className="text-sm font-medium">{t("sessionLoadFailed.title")}</p>
        <p className="mt-2 text-sm text-muted-foreground">
          {t("sessionLoadFailed.description")}
        </p>
        <Button className="mt-5" variant="outline" onClick={onRetry}>
          {t("actions.retry")}
        </Button>
      </div>
    </main>
  )
}
