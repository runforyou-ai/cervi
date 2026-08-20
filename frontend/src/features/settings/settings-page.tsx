/** 设置页。 */
import { useTranslation } from "react-i18next"

import {
  PagePaneLink,
  PagePaneNav,
  PageSplit,
} from "@/components/page-split"
import { PageHeader } from "@/components/page-header"
import { ProfileSettingsForm } from "@/features/settings/profile-settings-form"
import { StorageSettingsForm } from "@/features/settings/storage-settings-form"

/** 设置导航和当前设置表单。 */
export function SettingsPage({
  section,
}: {
  section: "profile" | "storage"
}) {
  const { t } = useTranslation("settings")
  const profile = section === "profile"

  return (
    <PageSplit
      paneWidth="md"
      paneVariant="nav"
      pane={
        <PagePaneNav label={t("navigationLabel")} title={t("title")}>
          <PagePaneLink to="/settings/profile">
            {t("navigation.profile")}
          </PagePaneLink>
          <PagePaneLink to="/settings/storage">
            {t("navigation.storage")}
          </PagePaneLink>
        </PagePaneNav>
      }
    >
      <PageHeader title={t(profile ? "profile.title" : "storage.title")} />
      <div className="min-h-0 flex-1 overflow-auto px-4 py-6 sm:px-6 lg:px-8">
        {profile ? <ProfileSettingsForm /> : <StorageSettingsForm />}
      </div>
    </PageSplit>
  )
}
