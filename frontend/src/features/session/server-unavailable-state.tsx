/** 企业服务器暂时不可用状态。 */
import { useTranslation } from "react-i18next"

import { Button } from "@/components/ui/button"

/** 展示连接失败状态和可选的服务器切换入口。 */
export function ServerUnavailableState({
  onRetry,
  onChangeServer,
}: {
  onRetry: () => void
  onChangeServer?: () => void
}) {
  const { t } = useTranslation("common")

  return (
    <main className="flex min-h-dvh items-center justify-center p-6">
      <div className="max-w-sm text-center">
        <p className="text-sm font-medium">
          {t("serverUnavailable.title")}
        </p>
        <p className="mt-2 text-sm text-muted-foreground">
          {t("serverUnavailable.description")}
        </p>
        <div className="mt-5 flex justify-center gap-2">
          <Button variant="outline" onClick={onRetry}>
            {t("actions.retry")}
          </Button>
          {onChangeServer ? (
            <Button variant="ghost" onClick={onChangeServer}>
              {t("actions.changeServer")}
            </Button>
          ) : null}
        </div>
      </div>
    </main>
  )
}
