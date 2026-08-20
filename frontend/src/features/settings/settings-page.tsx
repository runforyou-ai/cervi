/** 设置页。 */
import { useTranslation } from "react-i18next"
import { useOutletContext } from "react-router"

import type { Identity, User } from "@/api"
import {
  PagePaneLink,
  PagePaneNav,
  PageSplit,
} from "@/components/page-split"
import { PageHeader } from "@/components/page-header"
import { AppearanceSettings } from "@/features/settings/appearance-settings"
import { ChangePasswordForm } from "@/features/settings/change-password-form"
import { ProfileSettingsForm } from "@/features/settings/profile-settings-form"
import { StorageSettingsForm } from "@/features/settings/storage-settings-form"

type SettingsOutletContext = {
  identity: Identity
  updateUser: (user: User) => void
}

/** 设置导航和当前设置表单。 */
export function SettingsPage({
  section,
}: {
  section: "profile" | "password" | "appearance" | "storage"
}) {
  const { t } = useTranslation("settings")
  const { identity, updateUser } = useOutletContext<SettingsOutletContext>()
  const title = t(`${section}.title`)

  return (
    <PageSplit
      paneWidth="md"
      paneVariant="nav"
      pane={
        <PagePaneNav label={t("navigationLabel")} title={t("title")}>
          <PagePaneLink to="/settings/profile">
            {t("navigation.profile")}
          </PagePaneLink>
          <PagePaneLink to="/settings/password">
            {t("navigation.password")}
          </PagePaneLink>
          <PagePaneLink to="/settings/appearance">
            {t("navigation.appearance")}
          </PagePaneLink>
          <PagePaneLink to="/settings/storage">
            {t("navigation.storage")}
          </PagePaneLink>
        </PagePaneNav>
      }
    >
      <PageHeader title={title} />
      <div className="min-h-0 flex-1 overflow-auto px-4 py-6 sm:px-6 lg:px-8">
        {section === "profile" ? (
          <ProfileSettingsForm user={identity.user} onUpdated={updateUser} />
        ) : section === "password" ? (
          <ChangePasswordForm />
        ) : section === "appearance" ? (
          <AppearanceSettings />
        ) : (
          <StorageSettingsForm />
        )}
      </div>
    </PageSplit>
  )
}
