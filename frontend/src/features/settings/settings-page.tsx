/** 设置页。 */
import { useTranslation } from "react-i18next"
import { useOutletContext } from "react-router"

import {
  PagePaneLink,
  PagePaneNav,
  PageSplit,
} from "@/components/page-split"
import { PageHeader } from "@/components/page-header"
import { ChangePasswordForm } from "@/features/settings/change-password-form"
import { ProfileSettingsForm } from "@/features/settings/profile-settings-form"
import { StorageSettingsForm } from "@/features/settings/storage-settings-form"
import { UserPreferencesForm } from "@/features/settings/user-preferences-form"
import type { WorkspaceOutletContext } from "@/features/workspace/workspace-layout"

/** 个人设置导航和当前设置表单。 */
export function PersonalSettingsPage({
  section,
}: {
  section: "profile" | "security" | "preferences"
}) {
  const { t } = useTranslation("settings")
  const { identity, updateUser } = useOutletContext<WorkspaceOutletContext>()
  const title = t(`${section}.title`)

  return (
    <PageSplit
      paneWidth="md"
      paneVariant="nav"
      pane={
        <PagePaneNav
          label={t("personalNavigationLabel")}
          title={t("personalTitle")}
        >
          <PagePaneLink to="/account/profile">
            {t("navigation.profile")}
          </PagePaneLink>
          <PagePaneLink to="/account/security">
            {t("navigation.security")}
          </PagePaneLink>
          <PagePaneLink to="/account/preferences">
            {t("navigation.preferences")}
          </PagePaneLink>
        </PagePaneNav>
      }
    >
      <PageHeader title={title} />
      <div className="min-h-0 flex-1 overflow-auto px-4 py-6 sm:px-6 lg:px-8">
        {section === "profile" ? (
          <ProfileSettingsForm user={identity.user} onUpdated={updateUser} />
        ) : section === "security" ? (
          <ChangePasswordForm />
        ) : (
          <UserPreferencesForm user={identity.user} onUpdated={updateUser} />
        )}
      </div>
    </PageSplit>
  )
}

/** 系统设置导航和存储设置表单。 */
export function SystemSettingsPage() {
  const { t } = useTranslation("settings")

  return (
    <PageSplit
      paneWidth="md"
      paneVariant="nav"
      pane={
        <PagePaneNav
          label={t("systemNavigationLabel")}
          title={t("systemTitle")}
        >
          <PagePaneLink to="/settings/storage">
            {t("navigation.storage")}
          </PagePaneLink>
        </PagePaneNav>
      }
    >
      <PageHeader title={t("storage.title")} />
      <div className="min-h-0 flex-1 overflow-auto px-4 py-6 sm:px-6 lg:px-8">
        <StorageSettingsForm />
      </div>
    </PageSplit>
  )
}
