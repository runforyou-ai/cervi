/** 只读展示、就地编辑的表单字段。 */
import { type ComponentProps, type ReactNode, useState } from "react"
import {
  Controller,
  type Control,
  type FieldPathByValue,
  type FieldValues,
} from "react-hook-form"
import { useTranslation } from "react-i18next"

import { DetailEditRow } from "@/components/form/detail-edit-row"
import { Input } from "@/components/ui/input"

/** 默认展示字段值，悬停出现编辑入口，点击后就地编辑，失焦或回车退出编辑。 */
export function InlineEditField<T extends FieldValues>({
  control,
  name,
  label,
  required = false,
  format,
  compact,
  ...input
}: {
  control: Control<T>
  name: FieldPathByValue<T, string>
  label: string
  required?: boolean
  format?: (value: string) => ReactNode
  compact?: boolean
} & Omit<
  ComponentProps<typeof Input>,
  "name" | "value" | "defaultValue" | "onChange" | "onBlur" | "ref" | "required"
>) {
  const { t } = useTranslation("common")
  const [editing, setEditing] = useState(false)

  return (
    <Controller
      control={control}
      name={name}
      render={({ field, fieldState }) => (
        <DetailEditRow
          label={label}
          required={required}
          compact={compact}
          editing={editing}
          editEnabled={!editing}
          onEdit={() => setEditing(true)}
          value={
            field.value ? (
              (format?.(field.value) ?? field.value)
            ) : (
              <span className="text-muted-foreground">{t("notSet")}</span>
            )
          }
        >
          <Input
            {...input}
            autoFocus
            name={field.name}
            value={field.value}
            required={required}
            aria-invalid={fieldState.invalid}
            onChange={field.onChange}
            onBlur={() => {
              field.onBlur()
              setEditing(false)
            }}
            onKeyDown={(event) => {
              if (event.key === "Enter" || event.key === "Escape") {
                event.preventDefault()
                event.currentTarget.blur()
              }
            }}
            ref={field.ref}
          />
        </DetailEditRow>
      )}
    />
  )
}
