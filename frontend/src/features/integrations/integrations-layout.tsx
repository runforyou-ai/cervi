/** 集成页布局，左栏显示集成功能导航。 */
import { useTranslation } from "react-i18next"
import { Outlet } from "react-router"

import {
  PagePaneLink,
  PagePaneNav,
  PageSplit,
} from "@/components/page-split"

/** 显示集成导航和当前功能页面。 */
export function IntegrationsLayout() {
  const { t } = useTranslation("integrations")

  return (
    <PageSplit
      paneWidth="md"
      paneVariant="nav"
      pane={
        <PagePaneNav label={t("navigation")} title={t("title")}>
          <PagePaneLink to="/integrations/channels">
            {t("messageChannels")}
          </PagePaneLink>
          <PagePaneLink to="/integrations/model-services">
            {t("modelServices.navigation")}
          </PagePaneLink>
          <PagePaneLink to="/integrations/business-systems">
            {t("businessSystems")}
          </PagePaneLink>
          <PagePaneLink to="/integrations/connectors">
            {t("connectors.navigation")}
          </PagePaneLink>
          <PagePaneLink>{t("webhooks")}</PagePaneLink>
          <PagePaneLink>{t("openApi")}</PagePaneLink>
        </PagePaneNav>
      }
    >
      <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
        <Outlet />
      </div>
    </PageSplit>
  )
}
