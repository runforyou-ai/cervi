/** 偏好设置中的主题字段。 */
import { CheckIcon, MonitorIcon, MoonIcon, SunIcon } from "lucide-react"
import { useTranslation } from "react-i18next"

import { FieldLegend, FieldSet } from "@/components/ui/field"
import { cn } from "@/lib/utils"

/** 偏好设置支持的主题。 */
export const themePreferences = ["system", "light", "dark"] as const

export type ThemePreference = (typeof themePreferences)[number]

const themeOptions = [
  { value: "system", icon: MonitorIcon },
  { value: "light", icon: SunIcon },
  { value: "dark", icon: MoonIcon },
] as const satisfies ReadonlyArray<{
  value: ThemePreference
  icon: typeof MonitorIcon
}>

const themePreviewClasses = {
  system: {
    canvas: "bg-[linear-gradient(90deg,#fafafa_0_50%,#09090b_50%)]",
    sidebar: "bg-[linear-gradient(90deg,#f4f4f5_0_50%,#18181b_50%)]",
    line: "bg-zinc-400",
    panel: "bg-[linear-gradient(90deg,#fff_0_50%,#18181b_50%)]",
  },
  light: {
    canvas: "bg-zinc-50",
    sidebar: "bg-zinc-100",
    line: "bg-zinc-300",
    panel: "bg-white",
  },
  dark: {
    canvas: "bg-zinc-950",
    sidebar: "bg-zinc-900",
    line: "bg-zinc-600",
    panel: "bg-zinc-900",
  },
} as const satisfies Record<
  ThemePreference,
  { canvas: string; sidebar: string; line: string; panel: string }
>

/** 展示主题界面预览。 */
function ThemePreview({ theme }: { theme: ThemePreference }) {
  const classes = themePreviewClasses[theme]

  return (
    <div
      aria-hidden="true"
      className={cn(
        "flex h-24 overflow-hidden rounded-md border",
        classes.canvas,
      )}
    >
      <div className={cn("w-1/4 border-r p-2", classes.sidebar)}>
        <div className={cn("mb-2 size-3 rounded-full", classes.line)} />
        <div className={cn("mb-1 h-1.5 rounded-full", classes.line)} />
        <div className={cn("h-1.5 w-3/4 rounded-full", classes.line)} />
      </div>
      <div className="flex min-w-0 flex-1 flex-col gap-2 p-3">
        <div className={cn("h-2 w-1/2 rounded-full", classes.line)} />
        <div className={cn("flex-1 rounded border", classes.panel)} />
      </div>
    </div>
  )
}

/** 渲染主题选择字段。 */
export function AppearanceSettings({
  name,
  value,
  invalid,
  onBlur,
  onChange,
}: {
  name: string
  value: ThemePreference
  invalid: boolean
  onBlur: () => void
  onChange: (value: ThemePreference) => void
}) {
  const { t } = useTranslation("settings")

  return (
    <FieldSet className="w-full gap-0" aria-invalid={invalid}>
      <FieldLegend className="mb-0">{t("appearance.theme")}</FieldLegend>
      <div
        className="mt-4 grid gap-3 sm:grid-cols-3"
        data-slot="radio-group"
      >
        {themeOptions.map(({ value: optionValue, icon: Icon }) => {
          const checked = value === optionValue

          return (
            <label
              key={optionValue}
              className={cn(
                "min-w-0 cursor-pointer rounded-lg border-2 p-3 transition-[border-color,box-shadow]",
                "hover:border-foreground/30 has-focus-visible:border-ring",
                checked ? "border-primary" : "border-border",
              )}
            >
              <input
                className="sr-only"
                type="radio"
                name={name}
                value={optionValue}
                checked={checked}
                aria-invalid={invalid}
                onBlur={onBlur}
                onChange={() => onChange(optionValue)}
              />
              <ThemePreview theme={optionValue} />
              <span className="mt-3 flex items-center gap-2 text-sm font-medium">
                <Icon className="size-4 text-muted-foreground" />
                <span className="min-w-0 flex-1 truncate">
                  {t(`appearance.options.${optionValue}`)}
                </span>
                <CheckIcon
                  className={cn(
                    "size-4 text-primary transition-opacity",
                    checked ? "opacity-100" : "opacity-0",
                  )}
                />
              </span>
            </label>
          )
        })}
      </div>
    </FieldSet>
  )
}
