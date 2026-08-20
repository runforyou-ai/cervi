/** 页面返回入口。 */
import { ChevronLeftIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { Link } from "react-router"

/** 返回指定页面。 */
export function PageBack({ to }: { to: string }) {
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
