/** 通讯录表单共用的所属团队多选字段。 */
import { TeamCheckboxOptions } from "@/features/contacts/team-checkbox-options"
import type { Team } from "@/api"
import { Field, FieldDescription, FieldLabel } from "@/components/ui/field"

/** 展示团队选项并把勾选结果交回表单。 */
export function TeamCheckboxField({
  teams,
  label,
  emptyMessage,
  value,
  onChange,
  onBlur,
  disabled = false,
}: {
  teams: Team[]
  label: string
  emptyMessage: string
  value: string[]
  onChange: (ids: string[]) => void
  onBlur: () => void
  disabled?: boolean
}) {
  return (
    <Field>
      <FieldLabel>{label}</FieldLabel>
      {teams.length === 0 ? (
        <FieldDescription>{emptyMessage}</FieldDescription>
      ) : (
        <TeamCheckboxOptions
          disabled={disabled}
          teams={teams}
          value={value}
          onChange={onChange}
          onOptionBlur={onBlur}
        />
      )}
    </Field>
  )
}
