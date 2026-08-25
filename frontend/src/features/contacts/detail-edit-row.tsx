/** 联系人详情通用的字段级展示和编辑操作。 */
import type { ReactNode } from "react"
import { useTranslation } from "react-i18next"

import { Button } from "@/components/ui/button"

/** 详情行，支持进入字段编辑。 */
export function DetailEditRow({
  label,
  value,
  editing,
  editEnabled,
  onEdit,
  children,
}: {
  label: string
  value: ReactNode
  editing: boolean
  editEnabled: boolean
  onEdit: () => void
  children: ReactNode
}) {
  const { t } = useTranslation("contacts")
  const showEdit = !editing && editEnabled

  return (
    <div className="group rounded-md px-2 py-2.5 transition-colors hover:bg-muted/50 focus-within:bg-muted/50">
      <div className="flex min-h-9 items-start gap-3">
        <div className="w-28 shrink-0 pt-1 text-sm text-muted-foreground">
          {label}
        </div>
        <div className="min-w-0 flex-1">
          {editing ? (
            children
          ) : (
            <div className="pt-1 text-sm break-words">{value}</div>
          )}
        </div>
        <Button
          variant="ghost"
          size="sm"
          className={
            showEdit
              ? "opacity-100 sm:opacity-0 sm:group-hover:opacity-100 sm:group-focus-within:opacity-100"
              : "invisible"
          }
          disabled={!showEdit}
          aria-label={t("detail.editField", { field: label })}
          onClick={onEdit}
        >
          {t("detail.edit")}
        </Button>
      </div>
    </div>
  )
}

/** 详情字段编辑的保存和取消操作。 */
export function DetailEditActions({
  saving,
  onSave,
  onCancel,
}: {
  saving: boolean
  onSave: () => void
  onCancel: () => void
}) {
  const { t } = useTranslation("contacts")
  return (
    <div className="mt-3 flex items-center gap-2">
      <Button size="sm" disabled={saving} onClick={onSave}>
        {saving ? t("form.saving") : t("form.save")}
      </Button>
      <Button variant="outline" size="sm" disabled={saving} onClick={onCancel}>
        {t("form.cancel")}
      </Button>
    </div>
  )
}
