/** 通讯录表单共用的所属团队多选字段。 */
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
}: {
  teams: Team[]
  label: string
  emptyMessage: string
  value: string[]
  onChange: (ids: string[]) => void
  onBlur: () => void
}) {
  return (
    <Field>
      <FieldLabel>{label}</FieldLabel>
      {teams.length === 0 ? (
        <FieldDescription>{emptyMessage}</FieldDescription>
      ) : (
        <div className="grid gap-2 rounded-md border p-3 sm:grid-cols-2">
          {teams.map((team) => (
            <label key={team.id} className="flex items-center gap-2 text-sm">
              <input
                type="checkbox"
                className="size-4 accent-primary"
                checked={value.includes(team.id)}
                onBlur={onBlur}
                onChange={(event) =>
                  onChange(event.target.checked
                    ? [...value, team.id]
                    : value.filter((id) => id !== team.id))
                }
              />
              <span>{team.name}</span>
            </label>
          ))}
        </div>
      )}
    </Field>
  )
}
