/** 设置页。 */
import { useTranslation } from "react-i18next"

import {
  PagePaneLink,
  PagePaneNav,
  PageSplit,
} from "@/components/page-split"
import { StorageSettingsForm } from "@/features/settings/storage-settings-form"

/** 设置导航和对象存储表单。 */
export function SettingsPage() {
  const { t } = useTranslation("settings")

  return (
    <PageSplit
      paneWidth="sm"
      paneVariant="nav"
      pane={
        <PagePaneNav label={t("navigationLabel")} title={t("title")}>
          <PagePaneLink to="/settings/storage">
            {t("navigation.storage")}
          </PagePaneLink>
        </PagePaneNav>
      }
    >
      <div className="min-h-0 flex-1 overflow-auto px-4 py-6 sm:px-6 lg:px-8">
        <h2 className="text-xl font-semibold tracking-tight">
          {t("storage.title")}
        </h2>
        <StorageSettingsForm />
      </div>
    </PageSplit>
  )
}
