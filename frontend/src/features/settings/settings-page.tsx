/** 设置页。 */
import type { ReactNode } from "react"
import { useTranslation } from "react-i18next"

import { PageContent } from "@/components/page-content"
import { PagePaneLink, PagePaneNav, PageSplit } from "@/components/page-split"
import { PageHeader } from "@/components/page-header"
import { ChangePasswordForm } from "@/features/settings/change-password-form"
import { GeneralSettingsForm } from "@/features/settings/general-settings-form"
import { ProfileSettingsForm } from "@/features/settings/profile-settings-form"
import { RoleListPage } from "@/features/settings/role-list-page"
import { StorageSettingsForm } from "@/features/settings/storage-settings-form"
import { UserPreferencesForm } from "@/features/settings/user-preferences-form"
import { useWorkspace } from "@/features/workspace/workspace-context"

/** 个人设置导航和当前设置表单。 */
export function PersonalSettingsPage({
  section,
}: {
  section: "profile" | "security" | "preferences"
}) {
  const { t } = useTranslation("settings")
  const { identity, updateUser } = useWorkspace()
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
      <PageContent>
        {section === "profile" ? (
          <ProfileSettingsForm user={identity.user} onUpdated={updateUser} />
        ) : section === "security" ? (
          <ChangePasswordForm />
        ) : (
          <UserPreferencesForm user={identity.user} onUpdated={updateUser} />
        )}
      </PageContent>
    </PageSplit>
  )
}

/** 系统设置导航和当前设置表单。 */
export function SystemSettingsPage({
  section,
  children,
}: {
  section: "general" | "roles" | "storage"
  children?: ReactNode
}) {
  const { t } = useTranslation("settings")
  const { identity, updateOrganization } = useWorkspace()
  const title = t(`${section}.title`)

  return (
    <PageSplit
      paneWidth="md"
      paneVariant="nav"
      pane={
        <PagePaneNav
          label={t("systemNavigationLabel")}
          title={t("systemTitle")}
        >
          <PagePaneLink to="/settings/general">
            {t("navigation.general")}
          </PagePaneLink>
          <PagePaneLink to="/settings/roles">
            {t("navigation.roles")}
          </PagePaneLink>
          <PagePaneLink to="/settings/storage">
            {t("navigation.storage")}
          </PagePaneLink>
        </PagePaneNav>
      }
    >
      {section === "roles" ? (
        (children ?? <RoleListPage />)
      ) : (
        <>
          <PageHeader title={title} />
          <PageContent>
            {section === "general" ? (
              <GeneralSettingsForm
                organization={identity.organization}
                onUpdated={updateOrganization}
              />
            ) : (
              <StorageSettingsForm />
            )}
          </PageContent>
        </>
      )}
    </PageSplit>
  )
}
