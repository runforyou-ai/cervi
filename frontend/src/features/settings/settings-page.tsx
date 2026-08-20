/** 设置页。 */
import { useTranslation } from "react-i18next"
import { Link } from "react-router"

import {
  PagePaneLink,
  PagePaneNav,
  PageSplit,
} from "@/components/page-split"
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from "@/components/ui/breadcrumb"
import { StorageSettingsForm } from "@/features/settings/storage-settings-form"

/** 设置导航和对象存储表单。 */
export function SettingsPage() {
  const { t } = useTranslation("settings")

  return (
    <PageSplit
      paneWidth="sm"
      paneVariant="nav"
      pane={
        <PagePaneNav label={t("navigationLabel")}>
          <PagePaneLink to="/settings/storage">
            {t("navigation.storage")}
          </PagePaneLink>
        </PagePaneNav>
      }
    >
      <div className="min-h-0 flex-1 overflow-auto px-4 py-6 sm:px-6 lg:px-8">
        <Breadcrumb className="mb-6">
          <BreadcrumbList>
            <BreadcrumbItem>
              <BreadcrumbLink asChild>
                <Link to="/settings/storage">{t("title")}</Link>
              </BreadcrumbLink>
            </BreadcrumbItem>
            <BreadcrumbSeparator />
            <BreadcrumbItem>
              <BreadcrumbPage>{t("navigation.storage")}</BreadcrumbPage>
            </BreadcrumbItem>
          </BreadcrumbList>
        </Breadcrumb>
        <h2 className="text-xl font-semibold tracking-tight">
          {t("storage.title")}
        </h2>
        <StorageSettingsForm />
      </div>
    </PageSplit>
  )
}
