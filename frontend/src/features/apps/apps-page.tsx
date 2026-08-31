/** 应用启动器页，按平台打开企业配置的外部业务系统。 */
import { useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"

import {
  isApiError,
  listBusinessSystems,
  type BusinessSystem,
} from "@/api"
import { LoadingIndicator } from "@/components/loading-indicator"
import { PageContent } from "@/components/page-content"
import { PageHeader } from "@/components/page-header"
import { Button } from "@/components/ui/button"
import { OpenUrlDialog } from "@/features/apps/open-url-dialog"
import { useWorkspace } from "@/contexts/workspace-context"
import { resourceKeys } from "@/hooks/resource-keys"
import { useResource } from "@/hooks/use-resource"
import { apiErrorMessage } from "@/lib/form-errors"
import { recoverSession } from "@/lib/session-navigation"
import { openExternalPage } from "@/platform/open-external-page"

/** 陈列启用的业务系统并支持打开自定义网址。 */
export function AppsPage() {
  const { t } = useTranslation("apps")
  const navigate = useNavigate()
  const { identity } = useWorkspace()
  const [openUrlDialogOpen, setOpenUrlDialogOpen] = useState(false)
  const {
    data,
    loading,
    refreshing,
    error: loadError,
    refresh,
  } = useResource(resourceKeys.businessSystems(), () => listBusinessSystems())
  const showLoading = loading || (Boolean(loadError) && refreshing)
  const businessSystems = useMemo(
    () => (data?.businessSystems ?? []).filter((system) => system.enabled),
    [data],
  )

  /** 按平台打开选中的业务系统。 */
  async function openBusinessSystem(system: BusinessSystem) {
    try {
      await openExternalPage({ title: system.name, url: system.url })
      console.info("业务系统已打开", { business_system_id: system.id })
    } catch (error) {
      if (recoverSession(error, navigate)) return
      console.warn("业务系统打开失败", {
        business_system_id: system.id,
        error,
      })
      toast.error(isApiError(error) ? apiErrorMessage(error) : t("openError"))
    }
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
      <PageHeader title={t("title")}>
        {identity.organization.allowArbitraryUrl ? (
          <Button
            size="sm"
            variant="outline"
            onClick={() => setOpenUrlDialogOpen(true)}
          >
            {t("openUrl.action")}
          </Button>
        ) : null}
      </PageHeader>
      <PageContent>
        {showLoading ? (
          <LoadingIndicator className="min-h-48 justify-center rounded-lg border">
            {t("loading")}
          </LoadingIndicator>
        ) : loadError ? (
          <div className="flex min-h-48 flex-col items-center justify-center rounded-lg border text-center">
            <p className="text-sm text-muted-foreground">{t("loadError")}</p>
            <Button
              className="mt-4"
              variant="outline"
              onClick={() => void refresh()}
            >
              {t("retry")}
            </Button>
          </div>
        ) : businessSystems.length === 0 ? (
          <div className="flex min-h-48 flex-col items-center justify-center gap-1 rounded-lg border text-center">
            <p className="text-sm font-medium">{t("empty.title")}</p>
            <p className="text-sm text-muted-foreground">
              {t("empty.description")}
            </p>
          </div>
        ) : (
          <div className="grid grid-cols-[repeat(auto-fill,minmax(10rem,1fr))] gap-3">
            {businessSystems.map((system) => (
              <button
                key={system.id}
                type="button"
                title={system.name}
                aria-label={t("open", { name: system.name })}
                className="flex flex-col items-start gap-3 rounded-lg border bg-card p-4 text-left outline-none hover:bg-accent hover:text-accent-foreground focus-visible:ring-2 focus-visible:ring-ring"
                onClick={() => void openBusinessSystem(system)}
              >
                <span
                  aria-hidden="true"
                  className="flex size-10 items-center justify-center rounded-lg bg-primary/10 text-base font-semibold text-primary"
                >
                  {system.name.charAt(0).toUpperCase()}
                </span>
                <span className="flex w-full flex-col gap-0.5">
                  <span className="truncate text-sm font-medium">
                    {system.name}
                  </span>
                  {system.description ? (
                    <span className="line-clamp-2 text-xs text-muted-foreground">
                      {system.description}
                    </span>
                  ) : null}
                </span>
              </button>
            ))}
          </div>
        )}
      </PageContent>
      <OpenUrlDialog
        open={openUrlDialogOpen}
        onOpenChange={setOpenUrlDialogOpen}
      />
    </div>
  )
}
