/** 通讯录二级导航：同事、团队、外部联系人、我的助理四个入口。 */
import {
  BotIcon,
  ContactRoundIcon,
  UserRoundIcon,
  UsersIcon,
} from "lucide-react"
import { useTranslation } from "react-i18next"

import { PagePaneLink, PagePaneNav } from "@/components/page-split"

/** 渲染通讯录分类入口；助理随本地运行时交付，交付前显示为即将支持。 */
export function ContactScopeSidebar() {
  const { t } = useTranslation("contacts")

  return (
    <PagePaneNav label={t("scopeNavigation")} title={t("title")}>
      <PagePaneLink to="/contacts/employees" icon={UserRoundIcon}>
        {t("scopes.employees")}
      </PagePaneLink>
      <PagePaneLink to="/contacts/teams" icon={UsersIcon}>
        {t("scopes.teams")}
      </PagePaneLink>
      <PagePaneLink to="/contacts/external" icon={ContactRoundIcon}>
        {t("scopes.external")}
      </PagePaneLink>
      <PagePaneLink icon={BotIcon}>{t("scopes.assistants")}</PagePaneLink>
    </PagePaneNav>
  )
}
