/** 内页标题行右上角的返回入口。 */
import { ArrowLeftIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { Link } from "react-router"

import { Button } from "@/components/ui/button"

/** 返回所属列表页。 */
export function PageBackButton({ to }: { to: string }) {
  const { t } = useTranslation("common")
  const label = t("actions.back")

  return (
    <Button variant="ghost" size="icon-sm" asChild>
      <Link to={to} aria-label={label} title={label}>
        <ArrowLeftIcon />
      </Link>
    </Button>
  )
}
