/** 客服处理状态的共享文案。 */
import type { TFunction } from "i18next"
import { ServiceSessionStatus } from "@/api"

/** 客服处理状态文案。 */
export function sessionStatusLabel(
  status: ServiceSessionStatus,
  t: TFunction<"inbox">,
) {
  switch (status) {
    case ServiceSessionStatus.ServiceSessionStatusOpen:
      return t("sessionStatus.open")
    case ServiceSessionStatus.ServiceSessionStatusClosed:
      return t("sessionStatus.closed")
    default:
      console.warn("未知的客服处理状态", status)
      return ""
  }
}
