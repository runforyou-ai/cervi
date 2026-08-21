/** 工作状态的统一文案和语义化展示。 */
import type { ComponentProps } from "react"
import type { TFunction } from "i18next"
import { useTranslation } from "react-i18next"

import { WorkStatus } from "@/api"
import { cn } from "@/lib/utils"

/** 用户菜单允许主动切换的工作状态。 */
export const selectableWorkStatuses = [
  WorkStatus.WorkStatusWorking,
  WorkStatus.WorkStatusAway,
  WorkStatus.WorkStatusOffDuty,
] as const

/** 返回工作状态的本地化文案。 */
export function workStatusLabel(status: WorkStatus, t: TFunction<"common">) {
  switch (status) {
    case WorkStatus.WorkStatusWorking:
      return t("workStatuses.working")
    case WorkStatus.WorkStatusAway:
      return t("workStatuses.away")
    case WorkStatus.WorkStatusOffDuty:
      return t("workStatuses.offDuty")
    default:
      console.warn("未知的工作状态", status)
      return ""
  }
}

/** 返回工作状态点使用的语义颜色。 */
function workStatusDotClass(status: WorkStatus) {
  switch (status) {
    case WorkStatus.WorkStatusWorking:
      return "bg-success"
    case WorkStatus.WorkStatusAway:
      return "bg-warning"
    case WorkStatus.WorkStatusOffDuty:
      return "bg-muted-foreground"
    default:
      console.warn("未知的工作状态颜色", status)
      return ""
  }
}

/** 用小圆点显示当前工作状态。 */
export function WorkStatusDot({
  status,
  className,
  ...props
}: Omit<ComponentProps<"span">, "children"> & { status: WorkStatus }) {
  return (
    <span
      aria-hidden="true"
      className={cn(
        "inline-block size-2.5 shrink-0 rounded-full",
        workStatusDotClass(status),
        className,
      )}
      {...props}
    />
  )
}

/** 用带文字的徽标显示工作状态。 */
export function WorkStatusBadge({ status }: { status: WorkStatus }) {
  const { t } = useTranslation("common")

  return (
    <span className="inline-flex items-center gap-1.5 rounded-full bg-muted px-2 py-0.5 text-xs font-medium text-foreground">
      <WorkStatusDot status={status} className="size-1.5" />
      {workStatusLabel(status, t)}
    </span>
  )
}
