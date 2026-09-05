/** 移动端通讯录分类入口和后续功能占位。 */
import {
  BotIcon,
  ChevronRightIcon,
  ContactRoundIcon,
  UsersRoundIcon,
  UserRoundIcon,
} from "lucide-react"
import { useTranslation } from "react-i18next"
import { Link, Navigate, useParams } from "react-router"

import {
  MobilePageHeader,
  MobilePageState,
  MobileScrollArea,
} from "@/apps/mobile/mobile-page"

const categories = [
  { path: "employees", label: "employees", icon: UserRoundIcon },
  { path: "ai-employees", label: "agents", icon: BotIcon },
  { path: "teams", label: "teams", icon: UsersRoundIcon },
  { path: "external", label: "external", icon: ContactRoundIcon },
] as const

/** 展示通讯录四个分类的固定入口。 */
export function MobileContactsPage() {
  const { t } = useTranslation("mobile")
  return (
    <section className="flex h-full min-h-0 flex-col">
      <MobilePageHeader title={t("tabs.contacts")} />
      <MobileScrollArea storageKey="contacts">
        <ul className="divide-y">
          {categories.map(({ path, label, icon: Icon }) => (
            <li key={path}>
              <Link
                to={`/contacts/${path}`}
                state={{ mobileBack: true }}
                className="flex min-h-16 items-center gap-3 px-4 py-3 outline-none active:bg-muted focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring"
              >
                <span className="flex size-10 items-center justify-center rounded-xl bg-primary/10 text-primary">
                  <Icon className="size-5" />
                </span>
                <span className="min-w-0 flex-1 text-[15px] font-medium">
                  {t(`contacts.${label}`)}
                </span>
                <span className="text-xs text-muted-foreground">
                  {t("unavailable")}
                </span>
                <ChevronRightIcon className="size-4 text-muted-foreground" />
              </Link>
            </li>
          ))}
        </ul>
      </MobileScrollArea>
    </section>
  )
}

/** 明确提示分类尚未开放，不把未实现功能显示为空目录。 */
export function MobileContactCategoryPage() {
  const { t } = useTranslation("mobile")
  const { category } = useParams()
  const selected = categories.find((item) => item.path === category)
  if (!selected) return <Navigate to="/contacts" replace />
  return (
    <section className="flex h-full min-h-0 flex-col">
      <MobilePageHeader
        title={t(`contacts.${selected.label}`)}
        backTo="/contacts"
      />
      <MobilePageState
        title={t("unavailable")}
        description={t("contacts.unavailableDescription")}
      />
    </section>
  )
}
