/** 通讯录表单与详情共用的团队勾选区域。 */
import type { ComponentProps } from "react"

import type { Team } from "@/api"

/** 输出团队选择，并在保存期间保留焦点且阻止再次变更。 */
export function TeamCheckboxOptions({
  teams,
  value,
  onChange,
  onOptionBlur,
  disabled = false,
  preserveFocus = false,
  ...props
}: {
  teams: Team[]
  value: string[]
  onChange: (ids: string[]) => void
  onOptionBlur?: () => void
  disabled?: boolean
  preserveFocus?: boolean
} & Omit<ComponentProps<"div">, "onChange">) {
  return (
    <div className="grid gap-2 rounded-md border p-3 sm:grid-cols-2" {...props}>
      {teams.map((team) => (
        <label key={team.id} className="flex items-center gap-2 text-sm">
          <input
            type="checkbox"
            className="size-4 accent-primary disabled:cursor-wait disabled:opacity-60 aria-disabled:cursor-wait aria-disabled:opacity-60"
            disabled={disabled && !preserveFocus}
            aria-disabled={disabled}
            checked={value.includes(team.id)}
            onBlur={onOptionBlur}
            onChange={(event) => {
              if (disabled) return
              onChange(
                event.target.checked
                  ? [...value, team.id]
                  : value.filter((id) => id !== team.id),
              )
            }}
          />
          <span>{team.name}</span>
        </label>
      ))}
    </div>
  )
}
