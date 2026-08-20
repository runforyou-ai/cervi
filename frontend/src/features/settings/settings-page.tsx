/** 设置页，当前展示对象存储配置。 */
import { useTranslation } from "react-i18next"
import { Link, NavLink } from "react-router"

import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from "@/components/ui/breadcrumb"
import { StorageSettingsForm } from "@/features/settings/storage-settings-form"
import { cn } from "@/lib/utils"

/** 渲染设置导航和对象存储表单。 */
export function SettingsPage() {
  const { t } = useTranslation("settings")

  return (
    <section className="flex min-h-0 flex-1 flex-col md:flex-row">
      <aside className="w-full shrink-0 border-b p-4 md:w-52 md:border-r md:border-b-0">
        <nav aria-label={t("navigationLabel")}>
          <NavLink
            to="/settings/storage"
            className={({ isActive }) =>
              cn(
                "block rounded-md px-3 py-2 text-sm font-medium transition-colors",
                isActive
                  ? "bg-muted text-foreground"
                  : "text-muted-foreground hover:bg-muted/60 hover:text-foreground"
              )
            }
          >
            {t("navigation.storage")}
          </NavLink>
        </nav>
      </aside>

      <div className="min-w-0 flex-1 px-4 py-6 sm:px-6 lg:px-8">
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
    </section>
  )
}
