/** 设置页。 */
import type { ReactNode } from "react"
import {
  BrainCircuitIcon,
  Building2Icon,
  CodeXmlIcon,
  LockKeyholeIcon,
  MessagesSquareIcon,
  MonitorSmartphoneIcon,
  PlugIcon,
  ShieldCheckIcon,
  SlidersHorizontalIcon,
  UserRoundIcon,
  WebhookIcon,
} from "lucide-react"
import { useTranslation } from "react-i18next"

import { PageContent } from "@/components/page-content"
import {
  PagePaneGroup,
  PagePaneLink,
  PagePaneNav,
  PageSplit,
} from "@/components/page-split"
import { PageHeader } from "@/components/page-header"
import { ChangePasswordForm } from "@/features/settings/change-password-form"
import { DeviceListPage } from "@/features/settings/device-list-page"
import { GeneralSettingsForm } from "@/features/settings/general-settings-form"
import { ProfileSettingsForm } from "@/features/settings/profile-settings-form"
import { RoleListPage } from "@/features/roles/role-list-page"
import { UserPreferencesForm } from "@/features/settings/user-preferences-form"
import { useWorkspace } from "@/contexts/workspace-context"

/** 由设置外壳直接渲染表单的设置项。 */
const formSections = [
  "profile",
  "security",
  "preferences",
  "devices",
  "general",
] as const

type SettingsFormSection = (typeof formSections)[number]

export type SettingsSection =
  | SettingsFormSection
  | "roles"
  | "channels"
  | "modelServices"
  | "mcpServers"

/** 判断设置项的内容是否由设置外壳内的表单渲染。 */
function isFormSection(section: SettingsSection): section is SettingsFormSection {
  return (formSections as readonly string[]).includes(section)
}

/** 设置导航和当前设置页面。 */
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
    <PageSplit
      paneWidth="md"
      paneVariant="nav"
      pane={
        <PagePaneNav label={t("navigationLabel")} title={t("title")}>
          <PagePaneGroup title={t("groups.personal")}>
            <PagePaneLink to="/settings/profile" icon={UserRoundIcon}>
              {t("navigation.profile")}
            </PagePaneLink>
            <PagePaneLink to="/settings/security" icon={LockKeyholeIcon}>
              {t("navigation.security")}
            </PagePaneLink>
            <PagePaneLink to="/settings/preferences" icon={SlidersHorizontalIcon}>
              {t("navigation.preferences")}
            </PagePaneLink>
            <PagePaneLink to="/settings/devices" icon={MonitorSmartphoneIcon}>
              {t("navigation.devices")}
            </PagePaneLink>
          </PagePaneGroup>
          <PagePaneGroup title={t("groups.organization")}>
            <PagePaneLink to="/settings/general" icon={Building2Icon}>
              {t("navigation.general")}
            </PagePaneLink>
            <PagePaneLink to="/settings/roles" icon={ShieldCheckIcon}>
              {t("navigation.roles")}
            </PagePaneLink>
            <PagePaneLink to="/settings/channels" icon={MessagesSquareIcon}>
              {t("navigation.channels")}
            </PagePaneLink>
            <PagePaneLink
              to="/settings/model-services/chat"
              activePath="/settings/model-services"
              icon={BrainCircuitIcon}
            >
              {t("navigation.modelServices")}
            </PagePaneLink>
            <PagePaneLink to="/settings/mcp-servers" icon={PlugIcon}>
              {t("navigation.mcpServers")}
            </PagePaneLink>
            <PagePaneLink icon={WebhookIcon}>
              {t("navigation.webhooks")}
            </PagePaneLink>
            <PagePaneLink icon={CodeXmlIcon}>
              {t("navigation.openApi")}
            </PagePaneLink>
          </PagePaneGroup>
        </PagePaneNav>
      }
    >
      {children ? (
        children
      ) : isFormSection(section) ? (
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
      ) : (
        <RoleListPage />
      )}
    </PageSplit>
  )
}
