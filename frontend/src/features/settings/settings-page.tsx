/** 设置页内容；设置导航在工作台一级栏中显示。 */
import type { ReactNode } from "react"
import { useTranslation } from "react-i18next"

import { PageContent } from "@/components/page-content"
import { PageHeader } from "@/components/page-header"
import { ChangePasswordForm } from "@/features/settings/change-password-form"
import { DeviceListPage } from "@/features/settings/device-list-page"
import { GeneralSettingsForm } from "@/features/settings/general-settings-form"
import { ProfileSettingsForm } from "@/features/settings/profile-settings-form"
import { RoleListPage } from "@/features/roles/role-list-page"
import { UserPreferencesForm } from "@/features/settings/user-preferences-form"
import { useWorkspace } from "@/contexts/workspace-context"

export type SettingsSection =
  | "profile"
  | "security"
  | "preferences"
  | "devices"
  | "general"
  | "roles"

/** 渲染当前设置页面。 */
export function SettingsPage({
  section,
  children,
}: {
  section: SettingsSection
  children?: ReactNode
}) {
  const { t } = useTranslation("settings")
  const { identity } = useWorkspace()

  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
      {section === "roles" ? (
        (children ?? <RoleListPage />)
      ) : (
        <>
          <PageHeader title={t(`${section}.title`)} />
          <PageContent>
            {section === "profile" ? (
              <ProfileSettingsForm user={identity.user} />
            ) : section === "security" ? (
              <ChangePasswordForm />
            ) : section === "devices" ? (
              <DeviceListPage />
            ) : section === "general" ? (
              <GeneralSettingsForm organization={identity.organization} />
            ) : (
              <UserPreferencesForm user={identity.user} />
            )}
          </PageContent>
        </>
      )}
    </div>
  )
}
