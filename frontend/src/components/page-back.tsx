/** 页面返回入口。 */
import { useTranslation } from "react-i18next"
import { Link } from "react-router"

import { Button } from "@/components/ui/button"

/** 返回指定页面。 */
export function PageBack({ to }: { to: string }) {
  const { t } = useTranslation("common")

  return (
    <Button variant="outline" size="sm" asChild>
      <Link to={to}>{t("actions.back")}</Link>
    </Button>
  )
}
