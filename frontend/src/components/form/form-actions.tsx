/** 展示表单的保存、补充操作和取消入口。 */
import type { ReactNode } from "react"
import { LoaderCircleIcon } from "lucide-react"
import { useTranslation } from "react-i18next"
import { Link } from "react-router"

import { Button } from "@/components/ui/button"

/** 统一表单提交状态，允许在保存与取消之间插入业务操作。 */
export function FormActions({
  saving,
  disabled = false,
  cancelTo,
  children,
}: {
  saving: boolean
  disabled?: boolean
  cancelTo: string
  children?: ReactNode
}) {
  const { t } = useTranslation("common")
  return (
    <div className="flex items-center gap-2">
      <Button type="submit" disabled={saving || disabled}>
        {saving ? <LoaderCircleIcon className="animate-spin" /> : null}
        {saving ? t("actions.saving") : t("actions.save")}
      </Button>
      {children}
      <Button type="button" variant="outline" asChild>
        <Link to={cancelTo}>{t("actions.cancel")}</Link>
      </Button>
    </div>
  )
}
