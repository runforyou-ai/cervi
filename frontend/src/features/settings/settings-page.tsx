import { SettingsIcon } from "lucide-react"
import { useTranslation } from "react-i18next"

import { WorkspacePageHeader } from "@/features/workspace/workspace-page"

export function SettingsPage() {
  const { t } = useTranslation("settings")

  return (
    <>
      <WorkspacePageHeader title={t("title")} />
      <section className="flex min-h-0 flex-1 items-center justify-center p-6">
        <div className="max-w-sm text-center">
          <div className="mx-auto mb-4 flex size-11 items-center justify-center rounded-xl border bg-background shadow-sm">
            <SettingsIcon className="size-5 text-muted-foreground" />
          </div>
          <h2 className="text-base font-semibold tracking-tight">
            {t("placeholderTitle")}
          </h2>
          <p className="mt-2 text-sm leading-6 text-muted-foreground">
            {t("placeholderDescription")}
          </p>
        </div>
      </section>
    </>
  )
}
