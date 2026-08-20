/** 离开列表后的返回入口和页面标题。 */
import type { ReactNode } from "react"
import { ChevronLeftIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { Link } from "react-router"

/** 返回上一层列表。 */
export function PageBack({
  to,
}: {
  to: string
}) {
  const { t } = useTranslation("common")

  return (
    <Link
      to={to}
      className="inline-flex shrink-0 items-center gap-0.5 text-sm text-muted-foreground hover:text-foreground"
    >
      <ChevronLeftIcon className="size-4" />
      {t("actions.back")}
    </Link>
  )
}

/** 子页面顶部的返回和标题。 */
export function PageBackHeader({
  to,
  title,
}: {
  to: string
  title: ReactNode
}) {
  return (
    <div className="mb-6 flex items-center justify-between gap-3">
      <h2 className="text-xl font-semibold tracking-tight">{title}</h2>
      <PageBack to={to} />
    </div>
  )
}
